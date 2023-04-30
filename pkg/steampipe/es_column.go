package steampipe

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"

	"gitlab.com/keibiengine/keibi-engine/pkg/source"

	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-hclog"
	"github.com/turbot/go-kit/helpers"
	"github.com/turbot/go-kit/types"
	"github.com/turbot/steampipe-plugin-sdk/v4/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v4/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v4/plugin/context_key"
	"github.com/turbot/steampipe-plugin-sdk/v4/plugin/transform"
	"gitlab.com/keibiengine/steampipe-plugin-aws/aws"
	"gitlab.com/keibiengine/steampipe-plugin-azure/azure"
	"gitlab.com/keibiengine/steampipe-plugin-azuread/azuread"
)

func buildContext() context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, context_key.Logger, hclog.New(nil))
	return ctx
}

func AzureDescriptionToRecord(resource interface{}, indexName string) (map[string]*proto.Column, error) {
	return DescriptionToRecord(azure.Plugin(buildContext()), resource, indexName)
}

func AzureADDescriptionToRecord(resource interface{}, indexName string) (map[string]*proto.Column, error) {
	return DescriptionToRecord(azuread.Plugin(buildContext()), resource, indexName)
}

func AWSDescriptionToRecord(resource interface{}, indexName string) (map[string]*proto.Column, error) {
	return DescriptionToRecord(aws.Plugin(buildContext()), resource, indexName)
}

func AWSCells(indexName string) ([]string, error) {
	return Cells(aws.Plugin(buildContext()), indexName)
}
func AzureCells(indexName string) ([]string, error) {
	return Cells(azure.Plugin(buildContext()), indexName)
}
func AzureADCells(indexName string) ([]string, error) {
	return Cells(azuread.Plugin(buildContext()), indexName)
}
func Cells(plg *plugin.Plugin, indexName string) ([]string, error) {
	var cells []string
	table, ok := plg.TableMap[indexName]
	if !ok {
		return cells, fmt.Errorf("invalid index name: %s", indexName)
	}
	table.Plugin = plg
	for _, column := range table.Columns {
		if column != nil && column.Transform != nil {
			cells = append(cells, column.Name)
		} else {
			fmt.Println("column or transform is null", column, column.Transform)
		}
	}

	return cells, nil
}

func DescriptionToRecord(plg *plugin.Plugin, resource interface{}, indexName string) (map[string]*proto.Column, error) {
	cells := make(map[string]*proto.Column)
	ctx := buildContext()
	table, ok := plg.TableMap[indexName]
	if !ok {
		return cells, fmt.Errorf("invalid index name: %s", indexName)
	}
	table.Plugin = plg
	for _, column := range table.Columns {
		transformData := transform.TransformData{
			HydrateItem:    resource,
			HydrateResults: nil,
			ColumnName:     column.Name,
			KeyColumnQuals: nil,
		}

		if column != nil && column.Transform != nil {
			//value, err := column.Transform.Execute(ctx, &transformData, getDefaultColumnTransform(table, column))
			value, err := column.Transform.Execute(ctx, &transformData)
			if err != nil {
				return nil, err
			}

			c, err := interfaceToColumnValue(column, value)
			if err != nil {
				return nil, err
			}

			cells[column.Name] = c
		} else if column == nil {
			fmt.Println("column is null", indexName)
		} else if column.Transform == nil {
			fmt.Println("column transform is null", indexName, column.Name)
		}
	}

	return cells, nil
}

func getDefaultColumnTransform(t *plugin.Table, column *plugin.Column) *transform.ColumnTransforms {
	var columnTransform *transform.ColumnTransforms
	if defaultTransform := t.DefaultTransform; defaultTransform != nil {
		//did the table define a default transform
		columnTransform = defaultTransform
	} else if defaultTransform = t.Plugin.DefaultTransform; defaultTransform != nil {
		// maybe the plugin defined a default transform
		columnTransform = defaultTransform
	} else {
		// no table or plugin defined default transform - use the base default implementation
		// (just returning the field corresponding to the column name)
		columnTransform = &transform.ColumnTransforms{Transforms: []*transform.TransformCall{{Transform: transform.FieldValue, Param: column.Name}}}
	}
	return columnTransform
}

// convert a value of unknown type to a valid protobuf column value.type
func interfaceToColumnValue(column *plugin.Column, val interface{}) (*proto.Column, error) {
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("%s: %v", column.Name, r))
		}
	}()

	// if the value is a pointer, get its value and use that
	val = helpers.DereferencePointer(val)
	if val == nil {
		if column.Default != nil {
			val = column.Default
		} else {
			// return nil
			return &proto.Column{Value: &proto.Column_NullValue{}}, nil
		}
	}

	var columnValue *proto.Column

	switch column.Type {
	case proto.ColumnType_STRING:
		columnValue = &proto.Column{Value: &proto.Column_StringValue{StringValue: types.ToString(val)}}
		break
	case proto.ColumnType_BOOL:
		b, err := types.ToBool(val)
		if err != nil {
			return nil, fmt.Errorf("interfaceToColumnValue failed for column '%s': %v", column.Name, err)
		}
		columnValue = &proto.Column{Value: &proto.Column_BoolValue{BoolValue: b}}
		break
	case proto.ColumnType_INT:
		i, err := types.ToInt64(val)
		if err != nil {
			return nil, fmt.Errorf("interfaceToColumnValue failed for column '%s': %v", column.Name, err)
		}

		columnValue = &proto.Column{Value: &proto.Column_IntValue{IntValue: i}}
		break
	case proto.ColumnType_DOUBLE:
		d, err := types.ToFloat64(val)
		if err != nil {
			return nil, fmt.Errorf("interfaceToColumnValue failed for column '%s': %v", column.Name, err)
		}
		columnValue = &proto.Column{Value: &proto.Column_DoubleValue{DoubleValue: d}}
		break
	case proto.ColumnType_JSON:
		strValue, ok := val.(string)
		if ok {
			// NOTE: Strings are assumed to be raw JSON, so are passed through directly.
			// This is the most common case, but means it's currently impossible to
			// pass through a string and have it marshalled to be a JSON representation
			// of a string.
			columnValue = &proto.Column{Value: &proto.Column_JsonValue{JsonValue: []byte(strValue)}}
		} else {
			res, err := json.Marshal(val)
			if err != nil {
				log.Printf("[ERROR] failed to marshal value to json: %v\n", err)
				return nil, fmt.Errorf("%s: %v ", column.Name, err)
			}
			columnValue = &proto.Column{Value: &proto.Column_JsonValue{JsonValue: res}}
		}
	case proto.ColumnType_DATETIME, proto.ColumnType_TIMESTAMP:
		// cast val to time
		var timeVal, err = types.ToTime(val)
		if err != nil {
			return nil, fmt.Errorf("interfaceToColumnValue failed for column '%s': %v", column.Name, err)
		}
		// now convert time to protobuf timestamp
		timestamp, err := ptypes.TimestampProto(timeVal)
		if err != nil {
			return nil, fmt.Errorf("interfaceToColumnValue failed for column '%s': %v", column.Name, err)
		}
		columnValue = &proto.Column{Value: &proto.Column_TimestampValue{TimestampValue: timestamp}}
		break
	case proto.ColumnType_IPADDR:
		ipString := types.SafeString(val)
		// treat an empty string as a null ip address
		if ipString == "" {
			columnValue = &proto.Column{Value: &proto.Column_NullValue{}}
		} else {
			if ip := net.ParseIP(ipString); ip == nil {
				return nil, fmt.Errorf("%s: invalid ip address %s", column.Name, ipString)
			}
			columnValue = &proto.Column{Value: &proto.Column_IpAddrValue{IpAddrValue: ipString}}
		}
		break
	case proto.ColumnType_CIDR:
		cidrRangeString := types.SafeString(val)
		// treat an empty string as a null ip address
		if cidrRangeString == "" {
			columnValue = &proto.Column{Value: &proto.Column_NullValue{}}
		} else {
			if _, _, err := net.ParseCIDR(cidrRangeString); err != nil {
				return nil, fmt.Errorf("%s: invalid ip address %s", column.Name, cidrRangeString)
			}
			columnValue = &proto.Column{Value: &proto.Column_CidrRangeValue{CidrRangeValue: cidrRangeString}}
		}
		break
	default:
		return nil, fmt.Errorf("unrecognised columnValue type '%s'", column.Type)
	}

	return columnValue, nil

}

func SourceTypeByResourceType(resourceType string) source.Type {
	if strings.HasPrefix(strings.ToLower(resourceType), "aws") {
		return source.CloudAWS
	} else {
		return source.CloudAzure
	}
}

func ConvertToDescription(resourceType string, data interface{}) (interface{}, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	bs := string(b)
	for {
		idx := strings.Index(bs, ":{\"Time\":{}}")
		if idx < 0 {
			break
		}

		startIdx := idx - 1
		q := 0
		for i := idx - 1; i >= 0; i-- {
			if bs[i] == '"' {
				q++
				if q == 2 {
					startIdx = i
				}
			}
		}
		endIdx := idx + len(":{\"Time\":{}}")
		if bs[startIdx-1] == ',' {
			startIdx--
		} else if bs[endIdx] == ',' {
			endIdx++
		}
		bs = bs[:startIdx] + bs[endIdx:]
	}

	sourceType := SourceTypeByResourceType(resourceType)
	if sourceType == source.CloudAWS {
		var d interface{}
		for k, v := range AWSDescriptionMap {
			if strings.ToLower(resourceType) == strings.ToLower(k) {
				d = v
			}
		}
		err = json.Unmarshal([]byte(bs), d)
		if err != nil {
			log.Println("failed to unmarshal to description: ", bs)
			return nil, fmt.Errorf("unmarshalling: %v", err)
		}
		d = helpers.DereferencePointer(d)
		return d, nil
	} else {
		var d interface{}
		for k, v := range AzureDescriptionMap {
			if strings.ToLower(resourceType) == strings.ToLower(k) {
				d = v
			}
		}
		err = json.Unmarshal([]byte(bs), &d)
		if err != nil {
			log.Println("failed to unmarshal to description: ", bs)
			return nil, fmt.Errorf("unmarshalling: %v", err)
		}
		d = helpers.DereferencePointer(d)
		return d, nil
	}
}

//
//func JSONMarshal(v reflect.Value) (interface{}, error) {
//	// Indirect through pointers and interfaces
//	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
//		v = v.Elem()
//	}
//	fmt.Printf("Visiting %v (%v)\n", v, v.Kind())
//
//	switch v.Kind() {
//	case reflect.Array, reflect.Slice:
//		var res []interface{}
//		for i := 0; i < v.Len(); i++ {
//			r, err := JSONMarshal(v.Index(i))
//			if err != nil {
//				return nil, err
//			}
//			res = append(res, r)
//		}
//		return res, nil
//	case reflect.Struct:
//		for i := 0; i < v.Type().NumMethod(); i++ {
//			if v.Type().Method(i).Name == "MarshalJSON" {
//				fmt.Println("Found MarshalJSON", v)
//				return v.Interface(), nil
//			}
//		}
//
//		res := map[string]interface{}{}
//		for i := 0; i < v.NumField(); i++ {
//			if !v.Type().Field(i).IsExported() {
//				continue
//			}
//			if v.Type().Field(i).Tag.Get("json") == "-" {
//				continue
//			}
//
//			fmt.Println("field: ", v.Type().Field(i).Name)
//
//			r, err := JSONMarshal(v.Field(i))
//			if err != nil {
//				return nil, err
//			}
//
//			fieldName := v.Type().Field(i).Tag.Get("json")
//			if fieldName == "" {
//				fieldName = v.Type().Field(i).Name
//			}
//			res[fieldName] = r
//		}
//		return res, nil
//	case reflect.Map:
//		res := map[string]interface{}{}
//		for _, k := range v.MapKeys() {
//			r, err := JSONMarshal(v.MapIndex(k))
//			if err != nil {
//				return nil, err
//			}
//			res[k.String()] = r
//		}
//		return res, nil
//	default:
//		return v.Interface(), nil
//	}
//}
