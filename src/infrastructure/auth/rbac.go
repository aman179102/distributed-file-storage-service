package auth

import (
	"log/slog"
	"strings"

	"github.com/distributed-file-storage/service/src/domain"
)

type RBACEngine struct {
}

func NewRBACEngine() *RBACEngine {
	return &RBACEngine{}
}

func (e *RBACEngine) Authorize(user *domain.User, action domain.Action, resource string) *domain.PolicyEvaluation {
	if user.Role == "admin" {
		return &domain.PolicyEvaluation{Allowed: true, Reason: "admin access"}
	}

	requiredRole, ok := actionRoles[action]
	if !ok {
		return &domain.PolicyEvaluation{Allowed: false, Reason: "unknown action"}
	}

	if user.HasRole(requiredRole) || user.Can(requiredRole) {
		return &domain.PolicyEvaluation{Allowed: true, Reason: "role authorized"}
	}

	return &domain.PolicyEvaluation{Allowed: false, Reason: "insufficient role"}
}

func (e *RBACEngine) EvaluatePolicy(user *domain.User, action domain.Action, resource string, policies []*domain.IAMPolicy) *domain.PolicyEvaluation {
	if user.Role == "admin" {
		return &domain.PolicyEvaluation{Allowed: true, Reason: "admin access"}
	}

	for _, policy := range policies {
		for _, statement := range policy.Statements {
			if !e.matchAction(statement.Actions, action) {
				continue
			}
			if !e.matchResource(statement.Resources, resource) {
				continue
			}
			if statement.Condition != nil {
				if !e.evaluateCondition(statement.Condition) {
					continue
				}
			}
			if statement.Effect == domain.EffectDeny {
				return &domain.PolicyEvaluation{Allowed: false, Reason: "denied by policy"}
			}
			if statement.Effect == domain.EffectAllow {
				return &domain.PolicyEvaluation{Allowed: true, Reason: "allowed by policy"}
			}
		}
	}

	return &domain.PolicyEvaluation{Allowed: false, Reason: "no matching policy"}
}

func (e *RBACEngine) matchAction(policyActions []domain.Action, requestedAction domain.Action) bool {
	for _, a := range policyActions {
		if a == domain.ActionAll || a == requestedAction {
			return true
		}
	}
	return false
}

func (e *RBACEngine) matchResource(policyResources []string, resource string) bool {
	for _, r := range policyResources {
		if r == "*" || r == resource {
			return true
		}
		if strings.HasSuffix(r, "*") {
			prefix := strings.TrimSuffix(r, "*")
			if strings.HasPrefix(resource, prefix) {
				return true
			}
		}
	}
	return false
}

func (e *RBACEngine) evaluateCondition(condition *domain.Condition) bool {
	if condition.SecureTransport {
		slog.Warn("secure transport condition not implemented in evaluation")
	}
	return true
}

var actionRoles = map[domain.Action]string{
	domain.ActionListBuckets:   "readonly",
	domain.ActionCreateBucket:  "user",
	domain.ActionDeleteBucket:  "operator",
	domain.ActionGetBucket:     "readonly",
	domain.ActionListObjects:   "readonly",
	domain.ActionGetObject:     "readonly",
	domain.ActionPutObject:     "user",
	domain.ActionDeleteObject:  "operator",
	domain.ActionGetObjectACL:  "readonly",
	domain.ActionPutObjectACL:  "operator",
	domain.ActionMultipartUpload: "user",
}

func ValidateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return domain.NewInvalidInput("bucket name must be between 3 and 63 characters")
	}
	for _, c := range name {
		if !isValidBucketChar(c) {
			return domain.NewInvalidInput("bucket name can only contain lowercase letters, numbers, dots, and hyphens")
		}
	}
	return nil
}

func isValidBucketChar(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '.' || c == '-'
}
