package loglint

type Settings struct {
	Rules     RulesSettings     `mapstructure:"rules"`
	Sensitive SensitiveSettings `mapstructure:"sensitive"`
	Allowed   AllowedSettings   `mapstructure:"allowed"`
}

type RulesSettings struct {
	LowercaseStart bool `mapstructure:"lowercase_start"`
	EnglishOnly    bool `mapstructure:"english_only"`
	NoSpecial      bool `mapstructure:"no_special"`
	NoSensitive    bool `mapstructure:"no_sensitive"`
}

type SensitiveSettings struct {
	// Keywords are used in two places:
	// 1) match identifier / selector names inside dynamic message expressions
	// 2) match key-like prefixes in string literal parts (e.g. "api_key=")
	Keywords []string `mapstructure:"keywords"`

	// CheckLiterals makes the linter flag plain string literals containing keywords
	// (more strict, more false positives). Default: false.
	CheckLiterals bool `mapstructure:"check_literals"`
}

type AllowedSettings struct {
	// AllowPunct relaxes the strict "no punctuation" rule.
	// If true, the following are allowed: . , : ; ? (still disallows symbols/emoji and "!" by default)
	AllowPunct bool `mapstructure:"allow_punct"`
}

func DefaultSettings() Settings {
	return Settings{
		Rules: RulesSettings{
			LowercaseStart: true,
			EnglishOnly:    true,
			NoSpecial:      true,
			NoSensitive:    true,
		},
		Sensitive: SensitiveSettings{
			Keywords: []string{
				"password", "passwd", "pwd",
				"token",
				"api_key", "apikey",
				"secret",
				"private_key",
				"authorization", "bearer",
				"session",
				"jwt",
			},
			CheckLiterals: false,
		},
		Allowed: AllowedSettings{
			AllowPunct: false,
		},
	}
}
