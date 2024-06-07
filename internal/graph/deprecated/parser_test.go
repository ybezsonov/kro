package graph

import "testing"

const ()

func Test_parseDataCEL(t *testing.T) {
	type args struct {
		raw  []byte
		path string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "1",
			args: args{
				raw:  []byte(IRSAResourceGroupSpec),
				path: "defintion.spec.eksOIDC",
			},
			want:    "string",
			wantErr: false,
		},
		{
			name: "2",
			args: args{
				raw:  []byte(IRSAResourceGroupSpec),
				path: "defintion.spec.tags",
			},
			want:    "map[string]string",
			wantErr: false,
		},
		{
			name: "3",
			args: args{
				raw:  []byte(IRSAResourceB),
				path: "name",
			},
			want:    "service-account",
			wantErr: false,
		},
		{
			name: "4",
			args: args{
				raw:  []byte(IRSAResourceB),
				path: "definition.metadata.name",
			},
			want:    "${spec.serviceAccountName}",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDataCELFake(tt.args.raw, tt.args.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDataCEL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseDataCEL() = %v, want %v", got, tt.want)
			}
		})
	}
}
