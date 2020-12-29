package service

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/integration-system/isp-lib/v2/scripts"
	log_code "isp-script-service/codes"
	"isp-script-service/conf"
	"isp-script-service/domain"

	"github.com/integration-system/isp-lib/v2/config"
	log "github.com/integration-system/isp-log"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var Script = &scriptService{
	scriptEngine: scripts.NewEngine(),
}

type scriptService struct {
	store        atomic.Value
	scriptEngine *scripts.Engine
}

func (s *scriptService) ReceiveConfiguration(scriptDef []conf.ScriptDefinition) {
	newStore := make(map[string]scripts.Script)
	for i, value := range scriptDef {
		scr, err := s.Create(value.Script)
		if err != nil {
			log.Errorf(log_code.CreateScriptFromConfigError, "create script from config (number %d): %v", i, err)

			continue
		}
		newStore[value.Id] = scr
	}
	s.store.Store(newStore)
}

func (s *scriptService) Execute(req domain.ExecuteRequest) *domain.ScriptResp {
	scr, err := s.Create(req.Script)
	if err != nil {
		return s.respError(err, domain.ErrorCompile)
	}

	return s.executeScript(scr, req.Arg)
}

func (s *scriptService) ExecuteById(req domain.ExecuteByIdRequest) (*domain.ScriptResp, error) {
	scr, ok := s.store.Load().(map[string]scripts.Script)[req.Id]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "not defined script for id %s", req.Id)
	}

	return s.executeScript(scr, req.Arg), nil
}

func (s *scriptService) BatchExecute(req []domain.ExecuteByIdRequest) []domain.ScriptResp {
	wg := sync.WaitGroup{}
	store := s.store.Load().(map[string]scripts.Script)
	response := make([]domain.ScriptResp, len(req))
	for i := range req {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if scr, ok := store[req[i].Id]; !ok {
				response[i] = *s.respError(errors.Errorf("not defined script for id %s", req[i].Id), domain.ErrorCompile)
			} else {
				response[i] = *s.executeScript(scr, req[i].Arg)
			}
		}(i)
	}
	wg.Wait()

	return response
}

func (s *scriptService) BatchExecuteById(req domain.BatchExecuteByIdsRequest) []domain.ScriptResp {
	wg := sync.WaitGroup{}
	store := s.store.Load().(map[string]scripts.Script)
	response := make([]domain.ScriptResp, len(req.Ids))
	for i := range req.Ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if scr, ok := store[req.Ids[i]]; !ok {
				response[i] = *s.respError(errors.Errorf("not defined script for id %s", req.Ids[i]), domain.ErrorCompile)
			} else {
				response[i] = *s.executeScript(scr, req.Arg)
			}
		}(i)
	}
	wg.Wait()

	return response
}

func (*scriptService) Create(scr string) (scripts.Script, error) {
	cfg := config.GetRemote().(*conf.RemoteConfig)

	return scripts.NewScript([]byte(cfg.SharedScript),
		[]byte("(function() {\n"), []byte(scr), []byte("\n})();"))
}

var errEmpty = errors.New("empty answer, maybe lost return")

func (s *scriptService) executeScript(scr scripts.Script, arg interface{}) *domain.ScriptResp {
	cfg := config.GetRemote().(*conf.RemoteConfig)
	response, err := s.scriptEngine.Execute(scr, arg,
		scripts.WithScriptTimeout(time.Duration(cfg.ScriptExecutionTimeoutMs)*time.Millisecond))
	if err != nil {
		return s.respError(err, domain.ErrorRunTime)
	}
	if response == nil {
		return s.respError(errEmpty, domain.ErrorRunTime)
	}

	return &domain.ScriptResp{
		Result: response,
	}
}

func (*scriptService) respError(err error, errorType string) *domain.ScriptResp {
	respError := domain.Error{
		Type:        errorType,
		Description: err.Error(),
	}

	return &domain.ScriptResp{
		Error: &respError,
	}
}
