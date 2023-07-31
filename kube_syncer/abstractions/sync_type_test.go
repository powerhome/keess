package abstractions

import "testing"

func TestGetSyncType(t *testing.T) {
	type args struct {
		syncType string
	}
	tests := []struct {
		name string
		args args
		want SyncType
	}{
		{
			name: "namespace",
			args: args{
				syncType: "namespace",
			},
			want: Namespace,
		},
		{
			name: "cluster",
			args: args{
				syncType: "cluster",
			},
			want: Cluster,
		},
		{
			name: "invalid",
			args: args{
				syncType: "context",
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSyncType(tt.args.syncType); got != tt.want {
				t.Errorf("GetSyncType() = %v, want %v", got, tt.want)
			}
		})
	}
}
