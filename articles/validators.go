package articles

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/gosimple/slug"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
)

type ArticleModelValidator struct {
	Article struct {
		Title       string    `form:"title" json:"title" binding:"required"`
		Description string    `form:"description" json:"description" binding:"required"`
		Body        string    `form:"body" json:"body" binding:"required"`
		Tags        *[]string `form:"tagList" json:"tagList"`
	} `json:"article"`
	articleModel ArticleModel `json:"-"`
	isUpdate     bool
}

func NewArticleModelValidator() ArticleModelValidator {
	return ArticleModelValidator{}
}

func NewArticleModelValidatorFillWith(articleModel ArticleModel) ArticleModelValidator {
	articleModelValidator := NewArticleModelValidator()
	articleModelValidator.isUpdate = true
	articleModelValidator.Article.Title = articleModel.Title
	articleModelValidator.Article.Description = articleModel.Description
	articleModelValidator.Article.Body = articleModel.Body
	existingTags := make([]string, 0)
	for _, tagModel := range articleModel.Tags {
		existingTags = append(existingTags, tagModel.Tag)
	}
	articleModelValidator.Article.Tags = &existingTags
	return articleModelValidator
}

func (s *ArticleModelValidator) Bind(c *gin.Context) error {
	myUserModel := c.MustGet("my_user_model").(users.UserModel)

	err := common.Bind(c, s)
	if err != nil {
		return err
	}

	// For update: tagList: null is rejected (nil pointer after JSON unmarshal)
	if s.isUpdate && s.Article.Tags == nil {
		return errors.New("tagList cannot be null")
	}

	s.articleModel.Slug = slug.Make(s.Article.Title)
	s.articleModel.Title = s.Article.Title
	s.articleModel.Description = s.Article.Description
	s.articleModel.Body = s.Article.Body
	s.articleModel.Author = GetArticleUserModel(myUserModel)

	tags := []string{}
	if s.Article.Tags != nil {
		tags = *s.Article.Tags
	}
	s.articleModel.setTags(tags)
	return nil
}

type CommentModelValidator struct {
	Comment struct {
		Body string `form:"body" json:"body" binding:"required,max=2048"`
	} `json:"comment"`
	commentModel CommentModel `json:"-"`
}

func NewCommentModelValidator() CommentModelValidator {
	return CommentModelValidator{}
}

func (s *CommentModelValidator) Bind(c *gin.Context) error {
	myUserModel := c.MustGet("my_user_model").(users.UserModel)

	err := common.Bind(c, s)
	if err != nil {
		return err
	}
	s.commentModel.Body = s.Comment.Body
	s.commentModel.Author = GetArticleUserModel(myUserModel)
	return nil
}
