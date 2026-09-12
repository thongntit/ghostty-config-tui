package schema

import "testing"

func TestOptionValidateBoolean(t *testing.T) {
	option := Option{Key: "confirm-close-surface", Kind: KindBoolean}
	for _, value := range []string{"true", "false"} {
		if err := option.Validate(value); err != nil {
			t.Errorf("boolean %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"yes", "1", "TRUE"} {
		if err := option.Validate(value); err == nil {
			t.Errorf("invalid boolean %q accepted", value)
		}
	}
}

func TestOptionValidateNumberHonorsFiniteRange(t *testing.T) {
	min, max := 8.0, 32.0
	option := Option{Key: "font-size", Kind: KindNumber, Min: &min, Max: &max}
	for _, value := range []string{"8", "14", "32", " 16.5 "} {
		if err := option.Validate(value); err != nil {
			t.Errorf("number %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"NaN", "+Inf", "7", "33"} {
		if err := option.Validate(value); err == nil {
			t.Errorf("invalid number %q accepted", value)
		}
	}
}

func TestOptionValidateColorAcceptsGhosttyForms(t *testing.T) {
	option := Option{Key: "background", Kind: KindColor}
	for _, value := range []string{"282c34", "#282c34", "alice blue", "background", "cell-foreground"} {
		if err := option.Validate(value); err != nil {
			t.Errorf("color %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"#12", "#fff", "#gggggg", "rgb(1, 2, 3)"} {
		if err := option.Validate(value); err == nil {
			t.Errorf("invalid color %q accepted", value)
		}
	}
}

func TestOptionValidatePathAndDuration(t *testing.T) {
	path := Option{Key: "bell-audio-path", Kind: KindPath}
	for _, value := range []string{"~/Sounds/bell.wav", "$HOME/bell.wav", "/tmp/bell.wav"} {
		if err := path.Validate(value); err != nil {
			t.Errorf("path %q rejected: %v", value, err)
		}
	}
	if err := path.Validate("   "); err == nil {
		t.Error("blank path accepted")
	}

	duration := Option{Key: "click-repeat-interval", Kind: KindDuration}
	for _, value := range []string{"0", "0.2", "250ms", "1s 200ms", "1h30m"} {
		if err := duration.Validate(value); err != nil {
			t.Errorf("duration %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"-1s", "soon", "1 s nope"} {
		if err := duration.Validate(value); err == nil {
			t.Errorf("invalid duration %q accepted", value)
		}
	}
}

func TestOptionValidatePreservesUnknownExistingSyntaxBoundary(t *testing.T) {
	option := Option{Key: "theme", Kind: KindString}
	if err := option.Validate("theme-with spaces and=syntax"); err != nil {
		t.Fatalf("string value rejected: %v", err)
	}
	if err := option.Validate("line\nfeed"); err == nil {
		t.Fatal("unsafe string accepted")
	}
}
