package backend

import (
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/question"
)

// AnswerQuestion submits answers for a question. The returned bool
// reports whether this call resolved the pending request (true) or
// found it already resolved by a previous caller (false).
func (b *Backend) AnswerQuestion(workspaceID string, req proto.QuestionAnswer) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}

	responses := make([]question.Answer, len(req.Responses))
	for i, r := range req.Responses {
		responses[i] = question.Answer{
			QuestionID:  r.QuestionID,
			SelectedIDs: r.SelectedIDs,
			FillInText:  r.FillInText,
			Yes:         r.Yes,
			Notes:       r.Notes,
		}
	}

	if service, ok := ws.Questions.(interface {
		AnswerID(string, []question.Answer) bool
	}); ok {
		return service.AnswerID(req.BatchRequestID, responses), nil
	}
	return ws.Questions.Answer(responses), nil
}

// CancelQuestion cancels the pending question for a workspace.
// Returns true if a question was cancelled, false if none was
// pending.
func (b *Backend) CancelQuestion(workspaceID string, batchID ...string) (bool, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return false, err
	}
	if len(batchID) > 0 {
		if service, ok := ws.Questions.(interface{ CancelID(string) bool }); ok {
			return service.CancelID(batchID[0]), nil
		}
	}
	return ws.Questions.Cancel(), nil
}
