package goyze

import (
	errs "github.com/gomatic/go-error"
)

// Configuration errors.
const (
	// ErrUnknownSetting reports a setting name an analyzer does not define.
	ErrUnknownSetting errs.Const = "analyzer setting is not supported"
	// ErrInvalidSettingValue reports a value that a known setting rejects when
	// parsed or validated. The setting exists; only the value is bad.
	ErrInvalidSettingValue errs.Const = "analyzer setting value is invalid"
)

// SettingName is the name of an analyzer flag a Settings map targets.
type SettingName string

// SettingValue is the raw value assigned to a setting before the flag parses it.
type SettingValue string

// AnalyzerSettings maps each of one analyzer's setting names to its raw value.
type AnalyzerSettings map[SettingName]SettingValue

// Settings is the per-analyzer configuration: each analyzer name maps to that
// analyzer's settings. It is the public shape ApplyConfig consumes.
type Settings map[AnalyzerName]AnalyzerSettings

// ApplyConfig applies per-analyzer settings to the registrations' analyzer flags.
// settings is keyed by analyzer name, then by setting (flag) name. Unknown
// analyzer names are ignored (a config may target a larger suite than is
// present); an unknown setting name (ErrUnknownSetting) or an invalid value for a
// known setting (ErrInvalidSettingValue) is an error.
func ApplyConfig(regs []Registration, settings Settings) error {
	index := make(map[AnalyzerName]Registration, len(regs))
	for _, reg := range regs {
		index[reg.Name] = reg
	}
	for name, values := range settings {
		if reg, ok := index[name]; ok {
			if err := applySettings(reg, values); err != nil {
				return err
			}
		}
	}
	return nil
}

// applySettings sets each value on the registration's analyzer flags. A setting
// the analyzer does not define is ErrUnknownSetting; a defined setting whose value
// fails to parse or validate is ErrInvalidSettingValue.
func applySettings(reg Registration, values AnalyzerSettings) error {
	for key, value := range values {
		if reg.Analyzer.Flags.Lookup(string(key)) == nil {
			return ErrUnknownSetting.With(nil, "analyzer", reg.Name, "setting", key)
		}
		if err := reg.Analyzer.Flags.Set(string(key), string(value)); err != nil {
			return ErrInvalidSettingValue.With(err, "analyzer", reg.Name, "setting", key, "value", value)
		}
	}
	return nil
}
