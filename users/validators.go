package users

import (
	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
)

// *ModelValidator containing two parts:
// - Validator: write the form/json checking rule according to the doc https://github.com/go-playground/validator
// - DataModel: fill with data from Validator after invoking common.Bind(c, self)
// Then, you can just call model.save() after the data is ready in DataModel.
type UserModelValidator struct {
	User struct {
		Username *string `form:"username" json:"username" binding:"required,min=4,max=255"`
		Email    *string `form:"email" json:"email" binding:"required,email"`
		Password *string `form:"password" json:"password" binding:"required,min=8,max=255"`
		Bio      *string `form:"bio" json:"bio" binding:"omitempty,max=1024"`
		Image    *string `form:"image" json:"image" binding:"omitempty"`
	} `json:"user"`
	userModel UserModel `json:"-"`
}

// There are some difference when you create or update a model, you need to fill the DataModel before
// update so that you can use your origin data to cheat the validator.
// BTW, you can put your general binding logic here such as setting password.
func (self *UserModelValidator) Bind(c *gin.Context) error {
	err := common.Bind(c, self)
	if err != nil {
		return err
	}

	// Pointer fields: nil means JSON null (overrides pre-fill) → use zero value
	if self.User.Username != nil {
		self.userModel.Username = *self.User.Username
	}
	if self.User.Email != nil {
		self.userModel.Email = *self.User.Email
	}
	// Bio: nil means JSON null → store "" (serializer converts "" to JSON null)
	if self.User.Bio != nil {
		self.userModel.Bio = *self.User.Bio
	}
	// Image: nil means absent/null → keep existing; "" means clear
	if self.User.Image != nil && *self.User.Image != "" {
		self.userModel.Image = self.User.Image
	} else if self.User.Image != nil {
		self.userModel.Image = nil
	}

	// nil means JSON null (should have been rejected by required validation above)
	// Non-nil and != RandomPassword means a new password was provided
	if self.User.Password != nil && *self.User.Password != common.RandomPassword {
		self.userModel.setPassword(*self.User.Password)
	}
	return nil
}

// You can put the default value of a Validator here
func NewUserModelValidator() UserModelValidator {
	userModelValidator := UserModelValidator{}
	return userModelValidator
}

func NewUserModelValidatorFillWith(userModel UserModel) UserModelValidator {
	userModelValidator := NewUserModelValidator()

	// Pre-fill as pointers so JSON null can explicitly override
	username := userModel.Username
	userModelValidator.User.Username = &username
	email := userModel.Email
	userModelValidator.User.Email = &email
	bio := userModel.Bio
	userModelValidator.User.Bio = &bio
	userModelValidator.User.Image = userModel.Image
	rp := common.RandomPassword
	userModelValidator.User.Password = &rp
	return userModelValidator
}

type LoginValidator struct {
	User struct {
		Email    string `form:"email" json:"email" binding:"required,email"`
		Password string `form:"password" json:"password" binding:"required,min=8,max=255"`
	} `json:"user"`
	userModel UserModel `json:"-"`
}

func (self *LoginValidator) Bind(c *gin.Context) error {
	err := common.Bind(c, self)
	if err != nil {
		return err
	}

	self.userModel.Email = self.User.Email
	return nil
}

// You can put the default value of a Validator here
func NewLoginValidator() LoginValidator {
	loginValidator := LoginValidator{}
	return loginValidator
}
