package terminal

import "testing"

// testEnv builds an Env from a map. Keys present in the map are "set"
// (even if value is ""), keys absent are "not set".
func testEnv(m map[string]string) Env {
	return Env{
		Getenv: func(k string) string { return m[k] },
		LookupEnv: func(k string) (string, bool) {
			v, ok := m[k]
			return v, ok
		},
	}
}

func TestIsTTYNil(t *testing.T) {
	t.Parallel()
	if IsTTY(nil) {
		t.Error("IsTTY(nil) = true, want false")
	}
}

func TestCheckDimensions(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		width   int
		height  int
		wantErr bool
	}{
		"exact minimum":       {width: 80, height: 24, wantErr: false},
		"larger than minimum": {width: 120, height: 40, wantErr: false},
		"too narrow":          {width: 79, height: 24, wantErr: true},
		"too short":           {width: 80, height: 23, wantErr: true},
		"both too small":      {width: 40, height: 10, wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := CheckDimensions(tc.width, tc.height)
			if tc.wantErr && err == nil {
				t.Errorf("CheckDimensions(%d, %d) = nil, want error", tc.width, tc.height)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("CheckDimensions(%d, %d) = %v, want nil", tc.width, tc.height, err)
			}
		})
	}
}

func TestCheckDimensionsErrorMessage(t *testing.T) {
	t.Parallel()

	err := CheckDimensions(60, 20)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if len(msg) == 0 {
		t.Error("error message is empty")
	}
}

func TestDetectColorProfile(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		env  map[string]string
		want ColorProfile
	}{
		"NO_COLOR set":                     {env: map[string]string{"NO_COLOR": "1"}, want: ColorNone},
		"NO_COLOR empty string still disables": {env: map[string]string{"NO_COLOR": ""}, want: ColorNone},
		"COLORTERM truecolor":              {env: map[string]string{"COLORTERM": "truecolor"}, want: ColorTrueColor},
		"COLORTERM 24bit":                  {env: map[string]string{"COLORTERM": "24bit"}, want: ColorTrueColor},
		"TERM with 256color":               {env: map[string]string{"TERM": "xterm-256color"}, want: ColorANSI256},
		"bare xterm":                       {env: map[string]string{"TERM": "xterm"}, want: ColorBasic},
		"no env at all":                    {env: map[string]string{}, want: ColorBasic},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := DetectColorProfile(testEnv(tc.env))
			if got != tc.want {
				t.Errorf("DetectColorProfile() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsTmux(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		env  map[string]string
		want bool
	}{
		"TMUX set":     {env: map[string]string{"TMUX": "/tmp/tmux-1000/default,12345,0"}, want: true},
		"TMUX not set": {env: map[string]string{}, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := IsTmux(testEnv(tc.env))
			if got != tc.want {
				t.Errorf("IsTmux() = %v, want %v", got, tc.want)
			}
		})
	}
}
