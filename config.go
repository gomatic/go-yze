package goyze

import (
	errs "github.com/gomatic/go-error"
)

// ErrUnknownSetting reports a configuration setting an analyzer does not support.
const ErrUnknownSetting errs.Const = "analyzer setting is not supported"

// ApplyConfig applies per-analyzer settings to the registrations' analyzer flags.
// settings is keyed by analyzer name, then by setting (flag) name. Unknown
// analyzer names are ignored (a config may target a larger suite than is
// present); an unknown or invalid setting for a known analyzer is an error.
func ApplyConfig(regs []Registration, settings map[string]map[string]string) error {
	index := make(map[string]Registration, len(regs))
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

// applySettings sets each value on the registration's analyzer flags.
func applySettings(reg Registration, values map[string]string) error {
	for key, value := range values {
		if reg.Analyzer.Flags.Lookup(key) == nil {
			return ErrUnknownSetting.With(nil, "analyzer", reg.Name, "setting", key)
		}
		if err := reg.Analyzer.Flags.Set(key, value); err != nil {
			return ErrUnknownSetting.With(err, "analyzer", reg.Name, "setting", key)
		}
	}
	return nil
}
