package db

import (
	"errors"
	"fmt"
	"github.com/kaytu-io/kaytu-engine/pkg/describe/db/model"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"math/rand"
	"time"
)

func (db Database) CountComplianceJobsByDate(start time.Time, end time.Time) (int64, error) {
	var count int64
	tx := db.ORM.Model(&model.ComplianceJob{}).
		Where("status = ? AND updated_at >= ? AND updated_at < ?", model.ComplianceJobSucceeded, start, end).Count(&count)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, tx.Error
	}
	return count, nil
}

func (db Database) CreateComplianceJob(tx *gorm.DB, job *model.ComplianceJob) error {
	if tx == nil {
		tx = db.ORM
	}
	tx = tx.
		Model(&model.ComplianceJob{}).
		Create(job)
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func (db Database) UpdateComplianceJob(
	id uint, status model.ComplianceJobStatus, failureMsg string) error {
	tx := db.ORM.
		Model(&model.ComplianceJob{}).
		Where("id = ?", id).
		Updates(model.ComplianceJob{
			Status:         status,
			FailureMessage: failureMsg,
		})
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func (db Database) UpdateComplianceJobAreAllRunnersQueued(id uint, areAllRunnersQueued bool) error {
	tx := db.ORM.
		Model(&model.ComplianceJob{}).
		Where("id = ?", id).
		Updates(model.ComplianceJob{
			AreAllRunnersQueued: areAllRunnersQueued,
		})
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func (db Database) UpdateComplianceJobsTimedOut(complianceIntervalHours int64) error {
	tx := db.ORM.
		Model(&model.ComplianceJob{}).
		Where(fmt.Sprintf("created_at < NOW() - INTERVAL '%d HOURS'", complianceIntervalHours)).
		Where("status IN ?", []string{string(model.ComplianceJobCreated),
			string(model.ComplianceJobRunnersInProgress),
			string(model.ComplianceJobSummarizerInProgress),
		}).
		Updates(model.ComplianceJob{Status: model.ComplianceJobTimeOut, FailureMessage: "Job timed out"})
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func (db Database) GetComplianceJobByID(ID uint) (*model.ComplianceJob, error) {
	var job model.ComplianceJob
	tx := db.ORM.Where("id = ?", ID).Find(&job)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return &job, nil
}

func (db Database) CleanupComplianceJobsOlderThan(t time.Time) error {
	tx := db.ORM.Where("updated_at < ?", t).Unscoped().Delete(&model.ComplianceJob{})
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func (db Database) GetLastComplianceJob(benchmarkID string) (*model.ComplianceJob, error) {
	var job model.ComplianceJob
	tx := db.ORM.Model(&model.ComplianceJob{}).Where("benchmark_id = ?", benchmarkID).Order("created_at DESC").First(&job)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}
	return &job, nil
}

func (db Database) ListComplianceJobs() ([]model.ComplianceJob, error) {
	var job []model.ComplianceJob
	tx := db.ORM.Model(&model.ComplianceJob{}).First(&job)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}
	return job, nil
}

func (db Database) ListComplianceJobsForInterval(interval string) ([]model.ComplianceJob, error) {
	var job []model.ComplianceJob

	tx := db.ORM.Model(&model.ComplianceJob{}).Where(fmt.Sprintf("NOW() - updated_at < INTERVAL '%s'", interval)).Find(&job)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}
	return job, nil
}

func (db Database) ListComplianceJobsByConnectionID(connectionIds []string) ([]model.ComplianceJob, error) {
	var job []model.ComplianceJob
	tx := db.ORM.Model(&model.ComplianceJob{}).Where("connection_ids <@ ?", pq.Array(connectionIds)).Find(&job)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}
	return job, nil
}

func (db Database) ListPendingComplianceJobsByConnectionID(connectionIds []string) ([]model.ComplianceJob, error) {
	var job []model.ComplianceJob
	tx := db.ORM.Model(&model.ComplianceJob{}).
		Where("connection_ids <@ ?", pq.Array(connectionIds)).
		Where("status IN ?", []model.ComplianceJobStatus{model.ComplianceJobCreated, model.ComplianceJobRunnersInProgress}).
		Find(&job)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}
	return job, nil
}

func (db Database) ListComplianceJobsByBenchmarkID(benchmarkIds []string) ([]model.ComplianceJob, error) {
	var job []model.ComplianceJob
	tx := db.ORM.Model(&model.ComplianceJob{}).Where("benchmark_id IN ?", benchmarkIds).Find(&job)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, tx.Error
	}
	return job, nil
}

func (db Database) ListComplianceJobsByStatus(status model.ComplianceJobStatus) ([]model.ComplianceJob, error) {
	var jobs []model.ComplianceJob
	tx := db.ORM.Where("status = ?", status).Find(&jobs)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return jobs, nil
}

func (db Database) ListComplianceJobsByIds(ids []string) ([]model.ComplianceJob, error) {
	var jobs []model.ComplianceJob
	tx := db.ORM.Where("id IN ?", ids).Find(&jobs)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return jobs, nil
}

func (db Database) ListComplianceRunnersWithStatus(status model.ComplianceJobStatus) ([]model.ComplianceJob, error) {
	var jobs []model.ComplianceJob
	tx := db.ORM.Where("status = ?", status).Find(&jobs)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return jobs, nil
}

func (db Database) ListComplianceJobsWithUnqueuedRunners() ([]model.ComplianceJob, error) {
	var jobs []model.ComplianceJob
	tx := db.ORM.Where("are_all_runners_queued = ?", false).
		Where("status IN ?", []string{string(model.ComplianceJobCreated), string(model.ComplianceJobRunnersInProgress)}).
		Find(&jobs)
	if tx.Error != nil {
		return nil, tx.Error
	}
	// shuffle jobs
	rand.Shuffle(len(jobs), func(i, j int) {
		jobs[i], jobs[j] = jobs[j], jobs[i]
	})
	return jobs, nil
}

func (db Database) SetJobToRunnersInProgress() error {
	tx := db.ORM.Exec(`
UPDATE compliance_jobs j SET status = 'RUNNERS_IN_PROGRESS' WHERE status = 'CREATED' AND
	(select count(*) from compliance_runners where parent_job_id = j.id) > 0
`)
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func (db Database) ListJobsWithRunnersCompleted(manuals bool) ([]model.ComplianceJob, error) {
	var jobs []model.ComplianceJob

	query := `
SELECT * FROM compliance_jobs j WHERE status IN ('RUNNERS_IN_PROGRESS', 'SINK_IN_PROGRESS') AND are_all_runners_queued = TRUE AND
	(select count(*) from compliance_runners where parent_job_id = j.id AND 
	                                               NOT (status = 'SUCCEEDED' OR status = 'TIMEOUT' OR (status = 'FAILED' and retry_count >= 3))
	                                         ) = 0
`
	if manuals {
		query = query + ` AND trigger_type = ?`
	} else {
		query = query + ` AND trigger_type <> ?`
	}
	tx := db.ORM.Raw(query, model.ComplianceTriggerTypeManual).Find(&jobs)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return jobs, nil
}

func (db Database) GetLastUpdatedRunnerForParent(jobId uint) (model.ComplianceRunner, error) {
	var runner model.ComplianceRunner
	tx := db.ORM.Where("parent_job_id = ?", jobId).Order("updated_at DESC").First(&runner)
	if tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return model.ComplianceRunner{}, nil
		}
		return model.ComplianceRunner{}, tx.Error
	}

	return runner, nil
}

func (db Database) GetRunnersByParentJobID(jobID uint) ([]model.ComplianceRunner, error) {
	var runners []model.ComplianceRunner
	tx := db.ORM.Where("parent_job_id = ?", jobID).Find(&runners)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return runners, nil
}

func (db Database) FetchTotalFindingCountForComplianceJob(jobID uint) (int, error) {
	var count int
	tx := db.ORM.Raw(`SELECT coalesce(sum(coalesce(total_finding_count,0)), 0) FROM compliance_runners WHERE parent_job_id = ?`, jobID).Scan(&count)
	if tx.Error != nil {
		return 0, tx.Error
	}

	return count, nil
}

func (db Database) ListJobsToFinish() ([]model.ComplianceJob, error) {
	var jobs []model.ComplianceJob
	tx := db.ORM.Raw(`
SELECT * FROM compliance_jobs j WHERE status = 'SUMMARIZER_IN_PROGRESS' AND
	(select count(*) from compliance_summarizers where parent_job_id = j.id AND (status = 'SUCCEEDED' OR (status = 'FAILED' and retry_count >= 3))) > 0
`).Find(&jobs)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return jobs, nil
}

func (db Database) ListComplianceJobsByFilters(connectionId []string, benchmarkId []string, status []string,
	startTime time.Time, endTime *time.Time) ([]model.ComplianceJob, error) {
	var jobs []model.ComplianceJob
	tx := db.ORM.Model(&model.ComplianceJob{})

	if len(connectionId) > 0 {
		tx = tx.Where("connection_ids && ?", pq.Array(connectionId))
	}

	if len(benchmarkId) > 0 {
		tx = tx.Where("benchmark_id IN ?", benchmarkId)
	}
	if len(status) > 0 {
		tx = tx.Where("status IN ?", status)
	}
	tx = tx.Where("updated_at >= ?", startTime)
	if endTime != nil {
		tx = tx.Where("updated_at <= ?", *endTime)
	}

	tx = tx.Find(&jobs)
	if tx.Error != nil {
		return nil, tx.Error
	}

	return jobs, nil
}
