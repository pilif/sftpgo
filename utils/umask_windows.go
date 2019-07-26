package utils

import "github.com/pilif/sftpgo/logger"

// SetUmask does nothing on windows
func SetUmask(umask int, configValue string) {
	logger.Debug(logSender, "umask not available on windows, configured value %v (%v)", configValue, umask)
}
