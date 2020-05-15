package logger

func Debug(args ...interface{}) {
	if logger != nil {
		logger.Debug(args...)
	}
}

func Debugf(template string, args ...interface{}) {
	if logger != nil {
		logger.Debugf(template, args...)
	}
}

func Info(args ...interface{}) {
	if logger != nil {
		logger.Info(args...)
	}
}

func Infof(template string, args ...interface{}) {
	if logger != nil {
		logger.Infof(template, args...)
	}
}

func Warn(args ...interface{}) {
	if logger != nil {
		logger.Warn(args...)
	}
}

func Warnf(template string, args ...interface{}) {
	if logger != nil {
		logger.Warnf(template, args...)
	}
}

func Error(args ...interface{}) {
	if logger != nil {
		logger.Error(args...)
	}
}

func Errorf(template string, args ...interface{}) {
	if logger != nil {
		logger.Errorf(template, args...)
	}
}

func Fatal(args ...interface{}) {
	if logger != nil {
		logger.Fatal(args...)
	}
}

func Fatalf(template string, args ...interface{}) {
	if logger != nil {
		logger.Fatalf(template, args...)
	}
}

func Panic(args ...interface{}) {
	if logger != nil {
		logger.Panic(args...)
	}
}

func Panicf(template string, args ...interface{}) {
	if logger != nil {
		logger.Panicf(template, args...)
	}
}
