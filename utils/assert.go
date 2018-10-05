package utils

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// Assert b, if not, panic and log
func Assert(b bool, v ...interface{}) {
	if !b {
		p := fmt.Sprint(v...)
		log.Errorf("PANIC! reason: %v", p)
		panic(p)
	}
}

// Assertf b, if not, panic and log
func Assertf(b bool, format string, v ...interface{}) {
	if !b {
		p := fmt.Sprintf(format, v...)
		log.Errorf("PANIC! reason: %v", p)
		panic(p)
	}
}
