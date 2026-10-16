package question

import "github.com/SpherePrime/CLI/internal/pubsub"

func (s *questionService) Pending() *Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingRequest == nil || s.resolved {
		return nil
	}
	request := *s.pendingRequest
	request.Questions = append([]Question(nil), request.Questions...)
	return &request
}

func (s *questionService) AnswerID(id string, answers []Answer) bool {
	return s.resolveID(id, answers, false)
}

func (s *questionService) CancelID(id string) bool {
	return s.resolveID(id, nil, true)
}

func (s *questionService) resolveID(id string, answers []Answer, cancel bool) bool {
	s.mu.Lock()
	if s.pending == nil || s.resolved || (id != "" && id != s.pendingID) {
		s.mu.Unlock()
		return false
	}
	s.resolved = true
	batchID := s.pendingID
	if cancel {
		close(s.cancelled)
	} else {
		s.pending <- answers
	}
	s.mu.Unlock()
	s.notificationBroker.Publish(pubsub.CreatedEvent, Notification{BatchID: batchID})
	return true
}
