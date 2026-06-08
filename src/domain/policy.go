package domain

import "time"

type Effect string

const (
	EffectAllow Effect = "Allow"
	EffectDeny  Effect = "Deny"
)

type Action string

const (
	ActionListBuckets   Action = "s3:ListBuckets"
	ActionCreateBucket  Action = "s3:CreateBucket"
	ActionDeleteBucket  Action = "s3:DeleteBucket"
	ActionGetBucket     Action = "s3:GetBucket"
	ActionListObjects   Action = "s3:ListObjects"
	ActionGetObject     Action = "s3:GetObject"
	ActionPutObject     Action = "s3:PutObject"
	ActionDeleteObject  Action = "s3:DeleteObject"
	ActionGetObjectACL  Action = "s3:GetObjectAcl"
	ActionPutObjectACL  Action = "s3:PutObjectAcl"
	ActionMultipartUpload Action = "s3:MultipartUpload"
	ActionAll           Action = "s3:*"
)

type PolicyStatement struct {
	Effect    Effect     `json:"effect"`
	Actions   []Action   `json:"actions"`
	Resources []string   `json:"resources"`
	Condition *Condition `json:"condition,omitempty"`
}

type Condition struct {
	IPAddress    []string `json:"ip_address,omitempty"`
	DateBefore   string   `json:"date_before,omitempty"`
	DateAfter    string   `json:"date_after,omitempty"`
	SecureTransport bool `json:"secure_transport,omitempty"`
}

type IAMPolicy struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Statements []PolicyStatement `json:"statements"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type PolicyEvaluation struct {
	Allowed bool
	Reason  string
}

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	AccessKeyID  string    `json:"access_key_id,omitempty"`
	SecretKey    string    `json:"-" `
	Policies     []string  `json:"policies"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (u *User) HasRole(role string) bool {
	return u.Role == role
}

func (u *User) Can(requiredRole string) bool {
	roles := map[string]int{
		"admin":    100,
		"operator": 50,
		"user":     10,
		"readonly": 1,
	}
	userLevel, ok := roles[u.Role]
	if !ok {
		return false
	}
	requiredLevel, ok := roles[requiredRole]
	if !ok {
		return false
	}
	return userLevel >= requiredLevel
}
