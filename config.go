package main

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token       string `yaml:"token"`
	PollMs      int    `yaml:"poll_ms"`
	BaseURL     string `yaml:"base_url"`
	MaxHistory  int    `yaml:"max_history"`
	SimpleMode  bool   `yaml:"simple_mode"`
	ImageMode   string `yaml:"image_mode"`   // auto / chafa / kitty / ueberzug / none
	ImageHeight int    `yaml:"image_height"` // image height in terminal rows (default 8)
}

func defaultConfig() Config {
	return Config{
		Token:       "",
		PollMs:      200,
		BaseURL:     "https://prod-api.lolz.live",
		MaxHistory:  300,
		ImageMode:   "auto",
		ImageHeight: defaultImgHeight,
	}
}

func configPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.yml"
	}
	return filepath.Join(filepath.Dir(exe), "config.yml")
}

func loadConfig() (Config, bool) {
	cfg := defaultConfig()
	path := configPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, true
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), true
	}

	if cfg.Token == "" {
		return cfg, true
	}
	if cfg.PollMs < 50 {
		cfg.PollMs = 50
	}
	if cfg.MaxHistory < 50 {
		cfg.MaxHistory = 50
	}
	if cfg.ImageHeight < 3 {
		cfg.ImageHeight = defaultImgHeight
	}
	if cfg.ImageMode == "" {
		cfg.ImageMode = "auto"
	}

	return cfg, false
}

func saveConfig(cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	header := []byte("# Lolzchat TUI Configuration\n# Get your token from https://lolz.live/account/api\n#\n# simple_mode: false  — цветные ники с детектом групп, уников, радугой и т.д.\n# simple_mode: true   — все ники красные, только вы зеленым (как в старой версии)\n#\n# image_mode: auto      — авто-определение (kitty > ueberzug > chafa > none)\n#             chafa     — ANSI block-art (работает везде где есть chafa)\n#             kitty     — kitty graphics protocol (chafa --format=kitty)\n#             ueberzug  — Überzug++ пиксельный оверлей (для Alacritty и др.)\n#             none      — только ссылка на изображение\n# image_height: 8       — высота изображения в строках терминала\n\n")
	return os.WriteFile(configPath(), append(header, data...), 0600)
}
