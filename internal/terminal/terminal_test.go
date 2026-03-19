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
		"exact minimum":       {width: 40, height: 15, wantErr: false},
		"larger than minimum": {width: 120, height: 40, wantErr: false},
		"too narrow":          {width: 39, height: 15, wantErr: true},
		"too short":           {width: 40, height: 14, wantErr: true},
		"both too small":      {width: 20, height: 10, wantErr: true},
		"old minimum still ok": {width: 80, height: 24, wantErr: false},
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

	err := CheckDimensions(30, 10)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if len(msg) == 0 {
		t.Error("error message is empty")
	}
}

func TestLayoutTier(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		width      int
		height     int
		wantWidth  WidthTier
		wantHeight HeightTier
	}{
		// Width tier boundaries
		"width 120 = WideSplit":      {width: 120, height: 30, wantWidth: WidthWideSplit, wantHeight: HeightFooterVisible},
		"width 200 = WideSplit":      {width: 200, height: 30, wantWidth: WidthWideSplit, wantHeight: HeightFooterVisible},
		"width 119 = CompactSplit":   {width: 119, height: 30, wantWidth: WidthCompactSplit, wantHeight: HeightFooterVisible},
		"width 80 = CompactSplit":    {width: 80, height: 30, wantWidth: WidthCompactSplit, wantHeight: HeightFooterVisible},
		"width 79 = ModalOnly":       {width: 79, height: 30, wantWidth: WidthModalOnly, wantHeight: HeightFooterVisible},
		"width 60 = ModalOnly":       {width: 60, height: 30, wantWidth: WidthModalOnly, wantHeight: HeightFooterVisible},
		"width 59 = Minimal":         {width: 59, height: 30, wantWidth: WidthMinimal, wantHeight: HeightFooterVisible},
		"width 40 = Minimal":         {width: 40, height: 30, wantWidth: WidthMinimal, wantHeight: HeightFooterVisible},
		"width 39 = TooSmall":        {width: 39, height: 30, wantWidth: WidthTooSmall, wantHeight: HeightFooterVisible},
		"width 0 = TooSmall":         {width: 0, height: 30, wantWidth: WidthTooSmall, wantHeight: HeightFooterVisible},

		// Height tier boundaries
		"height 30 = FooterVisible":  {width: 80, height: 30, wantWidth: WidthCompactSplit, wantHeight: HeightFooterVisible},
		"height 50 = FooterVisible":  {width: 80, height: 50, wantWidth: WidthCompactSplit, wantHeight: HeightFooterVisible},
		"height 29 = Standard":       {width: 80, height: 29, wantWidth: WidthCompactSplit, wantHeight: HeightStandard},
		"height 24 = Standard":       {width: 80, height: 24, wantWidth: WidthCompactSplit, wantHeight: HeightStandard},
		"height 23 = Compact":        {width: 80, height: 23, wantWidth: WidthCompactSplit, wantHeight: HeightCompact},
		"height 15 = Compact":        {width: 80, height: 15, wantWidth: WidthCompactSplit, wantHeight: HeightCompact},
		"height 14 = TooSmall":       {width: 80, height: 14, wantWidth: WidthCompactSplit, wantHeight: HeightTooSmall},
		"height 0 = TooSmall":        {width: 80, height: 0, wantWidth: WidthCompactSplit, wantHeight: HeightTooSmall},

		// Combined edge cases
		"minimal width + compact height":  {width: 45, height: 18, wantWidth: WidthMinimal, wantHeight: HeightCompact},
		"too small both":                  {width: 30, height: 10, wantWidth: WidthTooSmall, wantHeight: HeightTooSmall},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH := LayoutTier(tc.width, tc.height)
			if gotW != tc.wantWidth {
				t.Errorf("LayoutTier(%d, %d) width tier = %d, want %d", tc.width, tc.height, gotW, tc.wantWidth)
			}
			if gotH != tc.wantHeight {
				t.Errorf("LayoutTier(%d, %d) height tier = %d, want %d", tc.width, tc.height, gotH, tc.wantHeight)
			}
		})
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
