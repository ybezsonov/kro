package schema

import (
	"reflect"
	"testing"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestOpenAPISchemaTransformer_Transform(t *testing.T) {
	type args struct {
		rawObject []byte
	}
	tests := []struct {
		name    string
		tr      *OpenAPISchemaTransformer
		args    args
		want    *extv1.JSONSchemaProps
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := &OpenAPISchemaTransformer{}
			got, err := tr.Transform(tt.args.rawObject, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenAPISchemaTransformer.Transform() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("OpenAPISchemaTransformer.Transform() = %v, want %v", got, tt.want)
			}
		})
	}
}
