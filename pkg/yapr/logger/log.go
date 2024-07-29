package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Level = zapcore.Level

const (
	DebugLevel = zapcore.DebugLevel
	InfoLevel  = zapcore.InfoLevel
	WarnLevel  = zapcore.WarnLevel
	ErrorLevel = zapcore.ErrorLevel
	PanicLevel = zapcore.PanicLevel
	FatalLevel = zapcore.FatalLevel
)

type Logger struct {
	logger *zap.Logger
	al     *zap.AtomicLevel
}

func New(level Level, fileOut io.Writer) *Logger {
	al := zap.NewAtomicLevelAt(level)

	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = zapcore.RFC3339TimeEncoder        // 将时间格式化为 RFC3339 格式
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder // 将日志级别显示为颜色
	cfg.EncodeCaller = zapcore.ShortCallerEncoder      // 显示完整的调用栈

	consoleEncoder := zapcore.NewConsoleEncoder(cfg)
	fileEncoder := zapcore.NewJSONEncoder(cfg)

	core := zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), al)

	if fileOut != nil {
		core = zapcore.NewTee(
			core,
			zapcore.NewCore(fileEncoder, zapcore.AddSync(fileOut), al),
		)
	}

	return &Logger{logger: zap.New(core).WithOptions(zap.AddCaller(), zap.AddCallerSkip(2)), al: &al}
}

func NewWithLogFile(level Level, path string) *Logger {
	fileWriter := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path,
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
	})

	return New(level, fileWriter)
}

func (l *Logger) SetLevel(level Level) {
	if l.al != nil {
		l.al.SetLevel(level)
	}
}

type Field = zap.Field

func (l *Logger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, fields...)
}

func (l *Logger) Debugf(msg string, args ...interface{}) {
	l.logger.Debug(fmt.Sprintf(msg, args...))
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, fields...)
}

func (l *Logger) Infof(msg string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(msg, args...))
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, fields...)
}

func (l *Logger) Warnf(msg string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(msg, args...))
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, fields...)
}

func (l *Logger) Errorf(msg string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(msg, args...))
}

func (l *Logger) Panic(msg string, fields ...Field) {
	l.logger.Panic(msg, fields...)
}

func (l *Logger) Panicf(msg string, args ...interface{}) {
	l.logger.Panic(fmt.Sprintf(msg, args...))
}

func (l *Logger) Fatal(msg string, fields ...Field) {
	l.logger.Fatal(msg, fields...)
}

func (l *Logger) Fatalf(msg string, args ...interface{}) {
	l.logger.Fatal(fmt.Sprintf(msg, args...))
}

func (l *Logger) Sync() error {
	return l.logger.Sync()
}

var std = New(InfoLevel, nil)

// Default 默认日志实例
func Default() *Logger {
	return std
}

// ReplaceDefault 替换默认日志实例
func ReplaceDefault(l *Logger) {
	std = l
}

// SetLevel 设置日志级别
func SetLevel(level Level) {
	std.SetLevel(level)
}

func Debug(msg string, fields ...Field) {
	std.Debug(msg, fields...)
}

func Debugf(msg string, args ...interface{}) {
	std.Debugf(msg, args...)
}

func Info(msg string, fields ...Field) {
	std.Info(msg, fields...)
}

func Infof(msg string, args ...interface{}) {
	std.Infof(msg, args...)
}

func Warn(msg string, fields ...Field) {
	std.Warn(msg, fields...)
}

func Warnf(msg string, args ...interface{}) {
	std.Warnf(msg, args...)
}

func Error(msg string, fields ...Field) {
	std.Error(msg, fields...)
}

func Errorf(msg string, args ...interface{}) {
	std.Errorf(msg, args...)
}

func Panic(msg string, fields ...Field) {
	std.Panic(msg, fields...)
}

func Panicf(msg string, args ...interface{}) {
	std.Panicf(msg, args...)
}

func Fatal(msg string, fields ...Field) {
	std.Fatal(msg, fields...)
}

func Fatalf(msg string, args ...interface{}) {
	std.Fatalf(msg, args...)
}

// Sync 同步日志
func Sync() error { return std.Sync() }
