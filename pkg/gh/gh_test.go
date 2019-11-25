package gh

import (
	"reflect"
	"testing"
	"time"
)

func TestOptions(t *testing.T) {
	type args struct {
		since      string
		all        bool
		utc        bool
		repo       string
		token      string
		output     string
		count      int
		allopts    bool
		milestones bool
	}

	tests := []struct {
		name    string
		args    args
		want    *Options
		wantErr bool
	}{
		{
			name: "unable to parse - default to 1970",
			args: args{
				since: "asdas",
				all:   true,
				utc:   true,
				repo:  "",
			},
			want: &Options{
				Since:     time.Unix(0, 0),
				AllIssues: true,
				TZ:        time.UTC,
				Repo:      "",
			},
			wantErr: true,
		},
		{
			name: "parse successful",
			args: args{
				since: "2018-12-09T09:09:09Z",
				all:   false,
				utc:   true,
				repo:  "s7evink/issues-to-go",
			},
			want: &Options{
				Since:     time.Date(2018, time.December, 9, 9, 9, 9, 0, time.UTC),
				AllIssues: false,
				TZ:        time.UTC,
				User:      "s7evink",
				Repo:      "issues-to-go",
			},
		},
		{
			name: "parse successful 2",
			args: args{
				since: "2018-12-09T09:09:09+01:00",
				all:   true,
				utc:   false,
				repo:  "s7evink/issues-to-go",
			},
			want: &Options{
				Since:     time.Date(2018, time.December, 9, 9, 9, 9, 0, time.Local),
				AllIssues: true,
				TZ:        time.Local,
				User:      "s7evink",
				Repo:      "issues-to-go",
			},
		},
		{
			name:    "parse all options with error",
			wantErr: true,
			args: args{
				since:   "2018-12-09T09:09:09+01:00",
				all:     true,
				utc:     false,
				repo:    "s7evink/issues-to-go",
				token:   "helloworld",
				output:  "./issues",
				count:   -1,
				allopts: true,
			},
			want: &Options{
				Since:      time.Date(2018, time.December, 9, 9, 9, 9, 0, time.Local),
				AllIssues:  true,
				TZ:         time.Local,
				User:       "s7evink",
				Repo:       "issues-to-go",
				Token:      "helloworld",
				OutputPath: "./issues",
				Milestones: true,
				Count:      -1,
			},
		},
		{
			name: "parse all options",
			args: args{
				since:      "2018-12-09T09:09:09+01:00",
				all:        true,
				utc:        false,
				repo:       "s7evink/issues-to-go",
				token:      "helloworld",
				output:     "./issues",
				count:      200,
				milestones: true,
				allopts:    true,
			},
			want: &Options{
				Since:      time.Date(2018, time.December, 9, 9, 9, 9, 0, time.Local),
				AllIssues:  true,
				TZ:         time.Local,
				User:       "s7evink",
				Repo:       "issues-to-go",
				Token:      "helloworld",
				OutputPath: "./issues",
				Milestones: true,
				Count:      200,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := []Option{
				Since(tt.args.since),
				All(tt.args.all),
				UTC(tt.args.utc),
				Repo(tt.args.repo),
			}
			if tt.args.allopts {
				o = append(o,
					Count(tt.args.count),
					Token(tt.args.token),
					Output(tt.args.output),
					Milestones(tt.args.milestones),
				)
			}
			opts := Options{}
			for _, opt := range o {
				if err := opt(&opts); err != nil && !tt.wantErr {
					t.Errorf("Lala")
				}
			}

			if !tt.wantErr && !reflect.DeepEqual(&opts, tt.want) {
				t.Errorf("Since() = %v, want %v", &opts, tt.want)
			}
		})
	}
}
