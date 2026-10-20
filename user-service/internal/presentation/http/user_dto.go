package http

type updateProfileRequest struct {
	DisplayName *string `json:"displayName" validate:"omitempty,min=1,max=100"`
	AvatarURL   *string `json:"avatarUrl" validate:"omitempty,url"`
}

type updateRoleRequest struct {
	Role string `json:"role" validate:"required,oneof=RIDER DRIVER ADMIN"`
}

type updateStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=ACTIVE SUSPENDED BANNED"`
}
