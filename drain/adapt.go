package drain

import (
	"encoding/json"
	"fmt"
	"github.com/jcelliott/lumber"
	"github.com/mu-box/microbox-logtap"
	"io"
)

type Publisher interface {
	Publish(tag []string, data string) error
}

func Filter(drain logtap.Drain, level int) logtap.Drain {
	return func(log logtap.Logger, msg logtap.Message) {
		if msg.Priority >= level {
			drain(log, msg)
		}
	}
}

func AdaptWriter(writer io.Writer) logtap.Drain {
	return func(log logtap.Logger, msg logtap.Message) {
		writer.Write([]byte(fmt.Sprintf("[%s][%s] <%d> %s\n", msg.Type, msg.Time, msg.Priority, msg.Content)))
	}
}

func AdaptPublisher(publisher Publisher) logtap.Drain {
	return func(log logtap.Logger, msg logtap.Message) {
		tags := []string{"log", msg.Type}
		severities := []string{"trace", "debug", "info", "warn", "error", "fatal"}
		tags = append(tags, severities[:((msg.Priority+1)%6)]...)
		data, err := json.Marshal(msg)
		if err != nil {
			return
		}
		publisher.Publish(tags, string(data))
	}
}

func AdaptLogger(logger logtap.Logger) logtap.Drain {
	return func(log logtap.Logger, msg logtap.Message) {
		switch msg.Priority {
		case lumber.TRACE:
			logger.Trace(msg.Content)
		case lumber.DEBUG:
			logger.Debug(msg.Content)
		case lumber.INFO:
			logger.Info(msg.Content)
		case lumber.WARN:
			logger.Warn(msg.Content)
		case lumber.ERROR:
			logger.Error(msg.Content)
		case lumber.FATAL:
			logger.Fatal(msg.Content)
		}
	}
}
