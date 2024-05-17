package ingestion

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu-engine/services/wastage/db/connector"
	"github.com/kaytu-io/kaytu-engine/services/wastage/db/model"
	"github.com/kaytu-io/kaytu-engine/services/wastage/db/repo"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	logger *zap.Logger

	DataAgeRepo repo.DataAgeRepo

	db                *connector.Database
	ec2InstanceRepo   repo.EC2InstanceTypeRepo
	rdsRepo           repo.RDSProductRepo
	rdsInstanceRepo   repo.RDSDBInstanceRepo
	ebsVolumeTypeRepo repo.EBSVolumeTypeRepo
	storageRepo       repo.RDSDBStorageRepo
}

func New(logger *zap.Logger, db *connector.Database, ec2InstanceRepo repo.EC2InstanceTypeRepo, rdsRepo repo.RDSProductRepo, rdsInstanceRepo repo.RDSDBInstanceRepo, storageRepo repo.RDSDBStorageRepo, ebsVolumeRepo repo.EBSVolumeTypeRepo, dataAgeRepo repo.DataAgeRepo) *Service {
	return &Service{
		logger:            logger,
		db:                db,
		ec2InstanceRepo:   ec2InstanceRepo,
		rdsInstanceRepo:   rdsInstanceRepo,
		rdsRepo:           rdsRepo,
		storageRepo:       storageRepo,
		ebsVolumeTypeRepo: ebsVolumeRepo,
		DataAgeRepo:       dataAgeRepo,
	}
}

func (s *Service) Start(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("paniced", zap.Error(fmt.Errorf("%v", r)))
			time.Sleep(15 * time.Minute)
			go s.Start(ctx)
		}
	}()

	ticker := time.NewTimer(2 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.logger.Info("checking data age")
		dataAge, err := s.DataAgeRepo.List()
		if err != nil {
			s.logger.Error("failed to list data age", zap.Error(err))
			time.Sleep(5 * time.Minute)
			continue
		}

		var ec2InstanceData *model.DataAge
		var rdsData *model.DataAge
		for _, data := range dataAge {
			data := data
			switch data.DataType {
			case "AWS::EC2::Instance":
				ec2InstanceData = &data
			case "AWS::RDS::Instance":
				rdsData = &data
			}
		}

		if ec2InstanceData == nil || ec2InstanceData.UpdatedAt.Before(time.Now().Add(-365*24*time.Hour)) {
			err = s.IngestEc2Instances(ctx)
			if err != nil {
				s.logger.Error("failed to ingest ec2 instances", zap.Error(err))
				time.Sleep(5 * time.Minute)
				continue
			}
			if ec2InstanceData == nil {
				err = s.DataAgeRepo.Create(&model.DataAge{
					DataType:  "AWS::EC2::Instance",
					UpdatedAt: time.Now(),
				})
				if err != nil {
					s.logger.Error("failed to create data age", zap.Error(err))
					time.Sleep(5 * time.Minute)
					continue
				}
			} else {
				err = s.DataAgeRepo.Update("AWS::EC2::Instance", model.DataAge{
					DataType:  "AWS::EC2::Instance",
					UpdatedAt: time.Now(),
				})
				if err != nil {
					s.logger.Error("failed to update data age", zap.Error(err))
					time.Sleep(5 * time.Minute)
					continue
				}
			}
		}

		if rdsData == nil || rdsData.UpdatedAt.Before(time.Now().Add(-7*24*time.Hour)) {
			err = s.IngestRDS()
			if err != nil {
				s.logger.Error("failed to ingest rds", zap.Error(err))
				time.Sleep(5 * time.Minute)
				continue
			}
			if rdsData == nil {
				err = s.DataAgeRepo.Create(&model.DataAge{
					DataType:  "AWS::RDS::Instance",
					UpdatedAt: time.Now(),
				})
				if err != nil {
					s.logger.Error("failed to create rds data age", zap.Error(err))
					time.Sleep(5 * time.Minute)
					continue
				}
			} else {
				err = s.DataAgeRepo.Update("AWS::RDS::Instance", model.DataAge{
					DataType:  "AWS::RDS::Instance",
					UpdatedAt: time.Now(),
				})
				if err != nil {
					s.logger.Error("failed to update rds data age", zap.Error(err))
					time.Sleep(5 * time.Minute)
					continue
				}
			}
		}
	}
}

func (s *Service) IngestEc2Instances(ctx context.Context) error {
	//transaction := s.db.Conn().Begin()
	//defer func() {
	//	transaction.Rollback()
	//}()
	var err error
	ec2InstanceTypeTable, err := s.ingestEc2InstancesBase(ctx, nil)
	if err != nil {
		s.logger.Error("failed to ingest ec2 instances", zap.Error(err))
		return err
	}

	err = s.ingestEc2InstancesExtra(ec2InstanceTypeTable, ctx, nil)
	if err != nil {
		s.logger.Error("failed to ingest ec2 instances extra", zap.Error(err))
		return err
	}

	//err = transaction.Commit().Error
	//if err != nil {
	//	s.logger.Error("failed to commit transaction", zap.Error(err))
	//	return err
	//}

	s.logger.Info("ingested ec2 instances")

	return nil
}

func (s *Service) ingestEc2InstancesBase(ctx context.Context, transaction *gorm.DB) (string, error) {
	ec2InstanceTypeTable := fmt.Sprintf("ec2_instance_types_%s", time.Now().Format("2006_01_02"))
	err := s.db.Conn().Table(ec2InstanceTypeTable).AutoMigrate(&model.EC2InstanceType{})
	if err != nil {
		s.logger.Error("failed to auto migrate",
			zap.String("table", ec2InstanceTypeTable),
			zap.Error(err))
		return ec2InstanceTypeTable, err
	}

	ebsVolumeTypeTable := fmt.Sprintf("ebs_volume_types_%s", time.Now().Format("2006_01_02"))
	err = s.db.Conn().Table(ebsVolumeTypeTable).AutoMigrate(&model.EBSVolumeType{})
	if err != nil {
		s.logger.Error("failed to auto migrate",
			zap.String("table", ebsVolumeTypeTable),
			zap.Error(err))
		return ec2InstanceTypeTable, err
	}

	url := "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/index.csv"
	resp, err := http.Get(url)
	if err != nil {
		return ec2InstanceTypeTable, err
	}
	csvr := csv.NewReader(resp.Body)
	csvr.FieldsPerRecord = -1

	var columns map[string]int
	for {
		values, err := csvr.Read()
		if err != nil {
			return ec2InstanceTypeTable, err
		}

		if len(values) > 2 {
			columns = readColumnPositions(values)
			break
		}
	}

	err = s.ec2InstanceRepo.Truncate(ec2InstanceTypeTable, transaction)
	if err != nil {
		return ec2InstanceTypeTable, err
	}

	err = s.ebsVolumeTypeRepo.Truncate(ebsVolumeTypeTable, transaction)
	if err != nil {
		return ec2InstanceTypeTable, err
	}
	// Read through each row in the CSV file and send a price.WithProduct on the results channel.
	for {
		row, err := csvr.Read()
		if err != nil {
			if err != io.EOF {
				return ec2InstanceTypeTable, err
			}
			break
		}

		switch row[columns["Product Family"]] {
		case "Compute Instance", "Compute Instance (bare metal)":
			v := model.EC2InstanceType{}
			v.PopulateFromMap(columns, row)

			if strings.ToLower(v.PhysicalProcessor) == "variable" {
				continue
			}
			if v.InstanceType == "" {
				continue
			}
			if v.TermType != "OnDemand" {
				continue
			}

			fmt.Println("Instance", v)
			err = s.ec2InstanceRepo.Create(ec2InstanceTypeTable, transaction, &v)
			if err != nil {
				return ec2InstanceTypeTable, err
			}
		case "Storage", "System Operation", "Provisioned Throughput":
			v := model.EBSVolumeType{}
			v.PopulateFromMap(columns, row)

			if v.VolumeType == "" {
				continue
			}
			if v.TermType != "OnDemand" {
				continue
			}
			fmt.Println("Volume", v)
			err = s.ebsVolumeTypeRepo.Create(ebsVolumeTypeTable, transaction, &v)
			if err != nil {
				return ec2InstanceTypeTable, err
			}
		}
	}

	err = s.ec2InstanceRepo.MoveViewTransaction(ec2InstanceTypeTable)
	if err != nil {
		return ec2InstanceTypeTable, err
	}

	err = s.ebsVolumeTypeRepo.MoveViewTransaction(ebsVolumeTypeTable)
	if err != nil {
		return ec2InstanceTypeTable, err
	}

	err = s.ec2InstanceRepo.RemoveOldTables(ec2InstanceTypeTable)
	if err != nil {
		return ec2InstanceTypeTable, err
	}

	err = s.ebsVolumeTypeRepo.RemoveOldTables(ebsVolumeTypeTable)
	if err != nil {
		return ec2InstanceTypeTable, err
	}

	return ec2InstanceTypeTable, nil
}

func (s *Service) ingestEc2InstancesExtra(ec2InstanceTypeTable string, ctx context.Context, transaction *gorm.DB) error {
	sdkConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		s.logger.Error("failed to load SDK config", zap.Error(err))
		return err
	}
	baseEc2Client := ec2.NewFromConfig(sdkConfig)

	regions, err := baseEc2Client.DescribeRegions(ctx, &ec2.DescribeRegionsInput{AllRegions: aws.Bool(false)})
	if err != nil {
		s.logger.Error("failed to describe regions", zap.Error(err))
		return err
	}

	for _, region := range regions.Regions {
		cnf, err := config.LoadDefaultConfig(ctx, config.WithRegion(*region.RegionName))
		if err != nil {
			s.logger.Error("failed to load SDK config", zap.Error(err), zap.String("region", *region.RegionName))
			return err
		}
		ec2Client := ec2.NewFromConfig(cnf)
		paginator := ec2.NewDescribeInstanceTypesPaginator(ec2Client, &ec2.DescribeInstanceTypesInput{})
		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				s.logger.Error("failed to get next page", zap.Error(err), zap.String("region", *region.RegionName))
				return err
			}
			for _, instanceType := range output.InstanceTypes {
				extras := getEc2InstanceExtrasMap(instanceType)
				if len(extras) == 0 {
					s.logger.Warn("no extras found", zap.String("region", *region.RegionName), zap.String("instanceType", string(instanceType.InstanceType)))
					continue
				}
				s.logger.Info("updating extras", zap.String("region", *region.RegionName), zap.String("instanceType", string(instanceType.InstanceType)), zap.Any("extras", extras))
				err = s.ec2InstanceRepo.UpdateExtrasByRegionAndType(ec2InstanceTypeTable, transaction, *region.RegionName, string(instanceType.InstanceType), extras)
				if err != nil {
					s.logger.Error("failed to update extras", zap.Error(err), zap.String("region", *region.RegionName), zap.String("instanceType", string(instanceType.InstanceType)))
					return err
				}
			}
		}
	}

	// Populate the still missing extras with the us-east-1 region data
	paginator := ec2.NewDescribeInstanceTypesPaginator(baseEc2Client, &ec2.DescribeInstanceTypesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			s.logger.Error("failed to get next page", zap.Error(err), zap.String("region", "all"))
			return err
		}
		for _, instanceType := range output.InstanceTypes {
			extras := getEc2InstanceExtrasMap(instanceType)
			if len(extras) == 0 {
				s.logger.Warn("no extras found", zap.String("region", "all"), zap.String("instanceType", string(instanceType.InstanceType)))
				continue
			}
			s.logger.Info("updating extras", zap.String("region", "all"), zap.String("instanceType", string(instanceType.InstanceType)), zap.Any("extras", extras))
			err = s.ec2InstanceRepo.UpdateNullExtrasByType(ec2InstanceTypeTable, transaction, string(instanceType.InstanceType), extras)
			if err != nil {
				s.logger.Error("failed to update extras", zap.Error(err), zap.String("region", "all"), zap.String("instanceType", string(instanceType.InstanceType)))
				return err
			}
		}
	}

	return nil
}

func (s *Service) IngestRDS() error {
	rdsInstancesTable := fmt.Sprintf("rdsdb_instances_%s", time.Now().Format("2006_01_02"))
	err := s.db.Conn().Table(rdsInstancesTable).AutoMigrate(&model.RDSDBInstance{})
	if err != nil {
		s.logger.Error("failed to auto migrate",
			zap.String("table", rdsInstancesTable),
			zap.Error(err))
		return err
	}
	rdsStorageTable := fmt.Sprintf("rdsdb_storages_%s", time.Now().Format("2006_01_02"))
	err = s.db.Conn().Table(rdsStorageTable).AutoMigrate(&model.RDSDBStorage{})
	if err != nil {
		s.logger.Error("failed to auto migrate",
			zap.String("table", rdsStorageTable),
			zap.Error(err))
		return err
	}
	rdsProductsTable := fmt.Sprintf("rds_products_%s", time.Now().Format("2006_01_02"))
	err = s.db.Conn().Table(rdsProductsTable).AutoMigrate(&model.RDSProduct{})
	if err != nil {
		s.logger.Error("failed to auto migrate",
			zap.String("table", rdsProductsTable),
			zap.Error(err))
		return err
	}

	url := "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonRDS/current/index.csv"
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	csvr := csv.NewReader(resp.Body)
	csvr.FieldsPerRecord = -1

	var columns map[string]int
	for {
		values, err := csvr.Read()
		if err != nil {
			return err
		}

		if len(values) > 2 {
			columns = readColumnPositions(values)
			break
		}
	}
	//
	//transaction := s.db.Conn().Begin()
	//defer func() {
	//	transaction.Rollback()
	//}()

	var transaction *gorm.DB

	err = s.rdsRepo.Truncate(rdsProductsTable, transaction)
	if err != nil {
		return err
	}
	err = s.rdsInstanceRepo.Truncate(rdsInstancesTable, transaction)
	if err != nil {
		return err
	}
	err = s.storageRepo.Truncate(rdsStorageTable, transaction)
	if err != nil {
		return err
	}
	// Read through each row in the CSV file and send a price.WithProduct on the results channel.
	for {
		row, err := csvr.Read()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}

		switch row[columns["Product Family"]] {
		case "Database Storage", "Provisioned IOPS", "Provisioned Throughput", "System Operation":
			v := model.RDSDBStorage{}
			v.PopulateFromMap(columns, row)

			if !v.DoIngest() {
				continue
			}

			fmt.Println("RDSDBStorage", v)

			err = s.storageRepo.Create(transaction, &v)
			if err != nil {
				return err
			}

		case "Database Instance":
			v := model.RDSDBInstance{}
			v.PopulateFromMap(columns, row)

			if v.TermType != "OnDemand" {
				continue
			}
			if v.LocationType == "AWS Outposts" {
				continue
			}

			fmt.Println("RDSDBInstance", v)

			err = s.rdsInstanceRepo.Create(transaction, &v)
			if err != nil {
				return err
			}

		default:
			v := model.RDSProduct{}
			v.PopulateFromMap(columns, row)

			if v.TermType != "OnDemand" {
				continue
			}
			if v.LocationType == "AWS Outposts" {
				continue
			}

			fmt.Println("RDS", v)

			err = s.rdsRepo.Create(transaction, &v)
			if err != nil {
				return err
			}
		}
	}
	//err = transaction.Commit().Error
	//if err != nil {
	//	return err
	//}
	return nil
}

func getEc2InstanceExtrasMap(instanceType ec2types.InstanceTypeInfo) map[string]any {
	extras := map[string]any{}
	if instanceType.EbsInfo != nil && instanceType.EbsInfo.EbsOptimizedInfo != nil {
		if instanceType.EbsInfo.EbsOptimizedInfo.BaselineBandwidthInMbps != nil {
			extras["ebs_baseline_bandwidth"] = *instanceType.EbsInfo.EbsOptimizedInfo.BaselineBandwidthInMbps
		}
		if instanceType.EbsInfo.EbsOptimizedInfo.MaximumBandwidthInMbps != nil {
			extras["ebs_maximum_bandwidth"] = *instanceType.EbsInfo.EbsOptimizedInfo.MaximumBandwidthInMbps
		}
		if instanceType.EbsInfo.EbsOptimizedInfo.BaselineIops != nil {
			extras["ebs_baseline_iops"] = *instanceType.EbsInfo.EbsOptimizedInfo.BaselineIops
		}
		if instanceType.EbsInfo.EbsOptimizedInfo.MaximumIops != nil {
			extras["ebs_maximum_iops"] = *instanceType.EbsInfo.EbsOptimizedInfo.MaximumIops
		}
		if instanceType.EbsInfo.EbsOptimizedInfo.BaselineThroughputInMBps != nil {
			extras["ebs_baseline_throughput"] = *instanceType.EbsInfo.EbsOptimizedInfo.BaselineThroughputInMBps
		}
		if instanceType.EbsInfo.EbsOptimizedInfo.MaximumThroughputInMBps != nil {
			extras["ebs_maximum_throughput"] = *instanceType.EbsInfo.EbsOptimizedInfo.MaximumThroughputInMBps
		}
	}
	return extras
}

// readColumnPositions maps column names to their position in the CSV file.
func readColumnPositions(values []string) map[string]int {
	columns := make(map[string]int)
	for i, v := range values {
		columns[v] = i
	}
	return columns
}
