package events

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/canonical/lxd/lxd/request"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/logger"
)

const ctxSecurityEvent = "security_event"

func PrepareSecurityEventContext(r *http.Request, serverName string, serverIP string, clusterID string, requestor *request.Requestor) {
	event := &LoggedSecurityEvent{
		EventSecurity: api.EventSecurity{
			AppID:                    "lxd",
			Type:                     "security",
			UserAgent:                r.UserAgent(),
			SourceIP:                 r.RemoteAddr,
			HostIP:                   serverIP,
			Hostname:                 r.URL.Hostname(),
			Protocol:                 r.URL.Scheme,
			Port:                     r.URL.Port(),
			RequestURI:               r.RequestURI,
			RequestMethod:            r.Method,
			UserID:                   requestor.CallerUsername(),
			UserAuthenticationMethod: requestor.CallerProtocol(),
			SessionID:                requestor.OIDCSessionID(),
			ClusterIdentifier:        clusterID,
			ClusterMemberName:        serverName,
		},
	}

	request.SetContextValue(r, ctxSecurityEvent, event)
}

type SecurityEvent interface {
	Name() string
	Description() string
	Level() logrus.Level
}

func FireSecurityEvent(ctx context.Context, e SecurityEvent) {
	ctxEvent, err := request.GetContextValue[*LoggedSecurityEvent](ctx, ctxSecurityEvent)
	if err != nil {
		logger.Error("A security event was triggered but no security event context was found", logger.Ctx{"err": err})
		return
	}

	lvl := e.Level()
	newEvent := *ctxEvent
	newEvent.Event = e.Name()
	newEvent.Description = e.Description()
	newEvent.Level = lvl.String()
	newEvent.Datetime = time.Now()

	var f func(msg string, ctx ...logger.Ctx)
	switch lvl {
	case logrus.TraceLevel:
		f = logger.Trace
	case logrus.DebugLevel:
		f = logger.Debug
	case logrus.InfoLevel:
		f = logger.Info
	case logrus.WarnLevel:
		f = logger.Warn
	case logrus.ErrorLevel:
		f = logger.Error
	default:
		f = logger.Info
	}

	f(newEvent.Description, logger.Ctx{"owasp_event": newEvent})
}

type LoggedSecurityEvent struct {
	api.EventSecurity
}

func (l LoggedSecurityEvent) String() string {
	b, _ := json.Marshal(l)
	return string(b)
}

type AuthNLoginFail struct{}

func (a AuthNLoginFail) Name() string {
	return "authn_login_fail"
}

func (a AuthNLoginFail) Description() string {
	return "Authentication failure on required endpoint"
}

func (a AuthNLoginFail) Level() logrus.Level {
	return logrus.WarnLevel
}
