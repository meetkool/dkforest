package template

import "testing"

func Test_buildTemplateName(t *testing.T) {
	type args struct {
		prefix string
		page   string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"", args{prefix: "/", page: "page"}, "page"},
		{"", args{prefix: "/admin", page: "page"}, "admin.page"},
		{"", args{prefix: "/admin/settings", page: "page"}, "admin.settings.page"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTemplateName(tt.args.prefix, tt.args.page); got != tt.want {
				t.Errorf("buildTemplateName() = %v, want %v", got, tt.want)
			}
		})
	}
}
