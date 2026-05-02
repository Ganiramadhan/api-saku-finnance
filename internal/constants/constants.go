package constants

const (
	// Auth
	MsgLogin          = "Login successful"
	MsgRegister       = "Registration successful"
	MsgChangePassword = "Password changed successfully"

	// Users
	MsgGetUsers        = "Successfully retrieved users"
	MsgGetUser         = "Successfully retrieved user"
	MsgCreateUser      = "Successfully created user"
	MsgUpdateUser      = "Successfully updated user"
	MsgDeleteUser      = "Successfully deleted user"
	MsgDeleteUserPhoto = "Successfully deleted user photo"
	MsgUploadImage     = "Image uploaded successfully"

	// Errors
	ErrInvalidRequest  = "Invalid request body"
	ErrInvalidUUID     = "Invalid UUID format"
	ErrUnauthorized    = "Authorization header is required"
	ErrInvalidToken    = "Invalid or expired token"
	ErrForbidden       = "You do not have permission to perform this action"
	ErrImageRequired   = "Image file is required"
	ErrInvalidFileType = "Invalid file type. Only images are allowed (jpeg, png, webp)"
	ErrFileTooLarge    = "File size too large. Maximum size is 5MB"
	ErrUploadFailed    = "Failed to upload image"
	ErrPreviewFailed   = "Failed to generate preview URL"

	// Limits
	MaxUploadSize = 5 * 1024 * 1024 // 5 MiB
)
