package analyzer

import (
	"testing"
	"time"

	"github.com/sonar-solutions/sonar-insights/internal/analyzer/bgtasks"
)

func ptr(t time.Time) *time.Time { return &t }

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestToEndOfDay(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "midnight input becomes 23:59:59",
			in:   time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC),
			want: time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC),
		},
		{
			name: "non-UTC input is normalised to UTC date",
			in:   time.Date(2024, 3, 15, 12, 30, 0, 0, time.FixedZone("EST", -5*3600)),
			want: time.Date(2024, 3, 15, 23, 59, 59, 0, time.UTC),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := toEndOfDay(tc.in)
			if !got.Equal(tc.want) {
				t.Errorf("toEndOfDay(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestFilterByDate(t *testing.T) {
	may10 := date(2024, time.May, 10)
	may11 := date(2024, time.May, 11)
	may12 := date(2024, time.May, 12)

	task := func(submittedAt time.Time) bgtasks.BgTask {
		return bgtasks.BgTask{SubmittedAt: submittedAt}
	}

	cases := []struct {
		name    string
		tasks   []bgtasks.BgTask
		from    *time.Time
		to      *time.Time
		wantLen int
	}{
		{
			name:    "to boundary: midnight on to-date is included",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 11, 0, 0, 0, 0, time.UTC))},
			to:      ptr(may11),
			wantLen: 1,
		},
		{
			name:    "to boundary: 23:59:59 on to-date is included",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 11, 23, 59, 59, 0, time.UTC))},
			to:      ptr(may11),
			wantLen: 1,
		},
		{
			name:    "to boundary: midnight on day after to-date is excluded",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 12, 0, 0, 0, 0, time.UTC))},
			to:      ptr(may11),
			wantLen: 0,
		},
		{
			name:    "from boundary: midnight on from-date is included",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 11, 0, 0, 0, 0, time.UTC))},
			from:    ptr(may11),
			wantLen: 1,
		},
		{
			name:    "from boundary: 23:59:59 on day before from-date is excluded",
			tasks:   []bgtasks.BgTask{task(time.Date(2024, time.May, 10, 23, 59, 59, 0, time.UTC))},
			from:    ptr(may11),
			wantLen: 0,
		},
		{
			name: "both bounds: tasks inside range are kept, outside are dropped",
			tasks: []bgtasks.BgTask{
				task(time.Date(2024, time.May, 9, 12, 0, 0, 0, time.UTC)),    // before from
				task(time.Date(2024, time.May, 10, 0, 0, 0, 0, time.UTC)),    // exactly from
				task(time.Date(2024, time.May, 11, 12, 0, 0, 0, time.UTC)),   // between
				task(time.Date(2024, time.May, 12, 23, 59, 59, 0, time.UTC)), // exactly to end
				task(time.Date(2024, time.May, 13, 0, 0, 0, 0, time.UTC)),    // after to
			},
			from:    ptr(may10),
			to:      ptr(may12),
			wantLen: 3,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterByDate(tc.tasks, tc.from, tc.to)
			if len(got) != tc.wantLen {
				t.Errorf("filterByDate returned %d tasks, want %d", len(got), tc.wantLen)
			}
		})
	}
}
