# 070: Change optional config key log level from WARN to DEBUG

Optional config keys (toolbox_mode, chord_prefix, etc.) log at WARN on startup when absent.
These are expected to be missing for most users and should not produce visible warnings.
Change the log.Warn call in cmd/config.go validateConfig() to log.Debug.
No behavioral change; only reduces default log noise.
