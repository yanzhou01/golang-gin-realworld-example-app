package users

import (
	"github.com/gin-gonic/gin"

	"github.com/gothinkster/golang-gin-realworld-example-app/common"
)

type ProfileSerializer struct {
	C *gin.Context
	UserModel
}

// Declare your response schema here
type ProfileResponse struct {
	ID        uint    `json:"-"`
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

// Put your response logic including wrap the userModel here.
func (self *ProfileSerializer) Response() ProfileResponse {
	myUserModel := self.C.MustGet("my_user_model").(UserModel)
	var bio *string
	if self.Bio != "" {
		bio = &self.Bio
	}
	var image *string
	if self.Image != nil && *self.Image != "" {
		image = self.Image
	}
	profile := ProfileResponse{
		ID:        self.ID,
		Username:  self.Username,
		Bio:       bio,
		Image:     image,
		Following: myUserModel.isFollowing(self.UserModel),
	}
	return profile
}

type UserSerializer struct {
	c *gin.Context
}

type UserResponse struct {
	Username string  `json:"username"`
	Email    string  `json:"email"`
	Bio      *string `json:"bio"`
	Image    *string `json:"image"`
	Token    string  `json:"token"`
}

func (self *UserSerializer) Response() UserResponse {
	myUserModel := self.c.MustGet("my_user_model").(UserModel)
	var bio *string
	if myUserModel.Bio != "" {
		bio = &myUserModel.Bio
	}
	var image *string
	if myUserModel.Image != nil && *myUserModel.Image != "" {
		image = myUserModel.Image
	}
	user := UserResponse{
		Username: myUserModel.Username,
		Email:    myUserModel.Email,
		Bio:      bio,
		Image:    image,
		Token:    common.GenToken(myUserModel.ID),
	}
	return user
}
