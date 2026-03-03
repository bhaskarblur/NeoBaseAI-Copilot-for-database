package vectordb

import (
	"context"
	"crypto/sha256"
	"fmt"

	pb "github.com/qdrant/go-client/qdrant"
)

// apiKeyCredentials implements grpc.PerRPCCredentials for API key auth.
type apiKeyCredentials struct {
	apiKey string
}

func (a *apiKeyCredentials) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{
		"api-key": a.apiKey,
	}, nil
}

func (a *apiKeyCredentials) RequireTransportSecurity() bool {
	return false
}

// pointIDToUUID converts a string PointID into a deterministic UUID v5-like format.
// This ensures the same logical ID always maps to the same Qdrant UUID.
func pointIDToUUID(id PointID) string {
	hash := sha256.Sum256([]byte(id))
	// Format as UUID: 8-4-4-4-12 hex
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		hash[0:4],
		hash[4:6],
		hash[6:8],
		hash[8:10],
		hash[10:16],
	)
}

// toQdrantValue converts a Go value to a Qdrant protobuf Value.
func toQdrantValue(v interface{}) *pb.Value {
	switch val := v.(type) {
	case string:
		return &pb.Value{
			Kind: &pb.Value_StringValue{StringValue: val},
		}
	case int:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{IntegerValue: int64(val)},
		}
	case int64:
		return &pb.Value{
			Kind: &pb.Value_IntegerValue{IntegerValue: val},
		}
	case float64:
		return &pb.Value{
			Kind: &pb.Value_DoubleValue{DoubleValue: val},
		}
	case float32:
		return &pb.Value{
			Kind: &pb.Value_DoubleValue{DoubleValue: float64(val)},
		}
	case bool:
		return &pb.Value{
			Kind: &pb.Value_BoolValue{BoolValue: val},
		}
	default:
		return &pb.Value{
			Kind: &pb.Value_StringValue{StringValue: fmt.Sprintf("%v", val)},
		}
	}
}

// fromQdrantValue converts a Qdrant protobuf Value back to a Go value.
func fromQdrantValue(v *pb.Value) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.GetKind().(type) {
	case *pb.Value_StringValue:
		return val.StringValue
	case *pb.Value_IntegerValue:
		return val.IntegerValue
	case *pb.Value_DoubleValue:
		return val.DoubleValue
	case *pb.Value_BoolValue:
		return val.BoolValue
	case *pb.Value_NullValue:
		return nil
	case *pb.Value_ListValue:
		list := make([]interface{}, 0, len(val.ListValue.GetValues()))
		for _, item := range val.ListValue.GetValues() {
			list = append(list, fromQdrantValue(item))
		}
		return list
	case *pb.Value_StructValue:
		m := make(map[string]interface{})
		for k, item := range val.StructValue.GetFields() {
			m[k] = fromQdrantValue(item)
		}
		return m
	default:
		return nil
	}
}
