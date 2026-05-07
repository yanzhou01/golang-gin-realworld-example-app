package articles

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gothinkster/golang-gin-realworld-example-app/common"
	"github.com/gothinkster/golang-gin-realworld-example-app/users"
	"gorm.io/gorm"
)

func ArticlesRegister(router *gin.RouterGroup) {
	router.GET("/feed", ArticleFeed)
	router.POST("", ArticleCreate)
	router.POST("/", ArticleCreate)
	router.PUT("/:slug", ArticleUpdate)
	router.PUT("/:slug/", ArticleUpdate)
	router.DELETE("/:slug", ArticleDelete)
	router.POST("/:slug/favorite", ArticleFavorite)
	router.DELETE("/:slug/favorite", ArticleUnfavorite)
	router.POST("/:slug/comments", ArticleCommentCreate)
	router.DELETE("/:slug/comments/:id", ArticleCommentDelete)
}

func ArticlesAnonymousRegister(router *gin.RouterGroup) {
	router.GET("", ArticleList)
	router.GET("/", ArticleList)
	router.GET("/:slug", ArticleRetrieve)
	router.GET("/:slug/comments", ArticleCommentList)
}

func TagsAnonymousRegister(router *gin.RouterGroup) {
	router.GET("", TagList)
	router.GET("/", TagList)
}

func ArticleCreate(c *gin.Context) {
	articleModelValidator := NewArticleModelValidator()
	if err := articleModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}

	var saveErr error
	for i := 0; i < 5; i++ {
		if saveErr = SaveOne(&articleModelValidator.articleModel); saveErr == nil {
			break
		}
		if isDup, field := common.IsDuplicateKeyError(saveErr); isDup && field == "slug" {
			articleModelValidator.articleModel.Slug = articleModelValidator.articleModel.Slug + "-" + common.RandString(6)
			articleModelValidator.articleModel.Model.ID = 0
			continue
		}
		break
	}
	if saveErr != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", saveErr))
		return
	}
	serializer := ArticleSerializer{c, articleModelValidator.articleModel}
	c.JSON(http.StatusCreated, gin.H{"article": serializer.Response()})
}

func ArticleList(c *gin.Context) {
	tag := c.Query("tag")
	author := c.Query("author")
	favorited := c.Query("favorited")
	limit := c.Query("limit")
	offset := c.Query("offset")
	articleModels, modelCount, err := FindManyArticle(tag, author, limit, offset, favorited)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("Invalid param")))
		return
	}
	serializer := ArticlesSerializer{c, articleModels}
	c.JSON(http.StatusOK, gin.H{"articles": serializer.Response(), "articlesCount": modelCount})
}

func ArticleFeed(c *gin.Context) {
	limit := c.Query("limit")
	offset := c.Query("offset")
	myUserModel := c.MustGet("my_user_model").(users.UserModel)
	if myUserModel.ID == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, common.NewError("token", errors.New("is missing")))
		return
	}
	articleUserModel := GetArticleUserModel(myUserModel)
	articleModels, modelCount, err := articleUserModel.GetArticleFeed(limit, offset)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("Invalid param")))
		return
	}
	serializer := ArticlesSerializer{c, articleModels}
	c.JSON(http.StatusOK, gin.H{"articles": serializer.Response(), "articlesCount": modelCount})
}

func ArticleRetrieve(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(&ArticleModel{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

func ArticleUpdate(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(&ArticleModel{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}
	myUserModel := c.MustGet("my_user_model").(users.UserModel)
	articleUserModel := GetArticleUserModel(myUserModel)
	if articleModel.AuthorID != articleUserModel.ID {
		c.JSON(http.StatusForbidden, common.NewError("article", errors.New("forbidden")))
		return
	}

	articleModelValidator := NewArticleModelValidatorFillWith(articleModel)
	if err := articleModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}

	db := common.GetDB()
	if err := db.Model(&articleModel).Updates(map[string]interface{}{
		"slug":        articleModelValidator.articleModel.Slug,
		"title":       articleModelValidator.articleModel.Title,
		"description": articleModelValidator.articleModel.Description,
		"body":        articleModelValidator.articleModel.Body,
	}).Error; err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	if err := db.Model(&articleModel).Association("Tags").Replace(articleModelValidator.articleModel.Tags); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}

	// Reload with updated associations
	articleModel, _ = FindOneArticle(&ArticleModel{Slug: articleModelValidator.articleModel.Slug})
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

func ArticleDelete(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(&ArticleModel{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}
	myUserModel := c.MustGet("my_user_model").(users.UserModel)
	articleUserModel := GetArticleUserModel(myUserModel)
	if articleModel.AuthorID != articleUserModel.ID {
		c.JSON(http.StatusForbidden, common.NewError("article", errors.New("forbidden")))
		return
	}
	if err := DeleteArticleModel(&ArticleModel{Slug: slug}); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	c.Status(http.StatusNoContent)
}

func ArticleFavorite(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(&ArticleModel{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}
	myUserModel := c.MustGet("my_user_model").(users.UserModel)
	if err = articleModel.favoriteBy(GetArticleUserModel(myUserModel)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

func ArticleUnfavorite(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(&ArticleModel{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}
	myUserModel := c.MustGet("my_user_model").(users.UserModel)
	if err = articleModel.unFavoriteBy(GetArticleUserModel(myUserModel)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	serializer := ArticleSerializer{c, articleModel}
	c.JSON(http.StatusOK, gin.H{"article": serializer.Response()})
}

func ArticleCommentCreate(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(&ArticleModel{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}
	commentModelValidator := NewCommentModelValidator()
	if err := commentModelValidator.Bind(c); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewValidatorError(err))
		return
	}
	commentModelValidator.commentModel.Article = articleModel

	if err := SaveOne(&commentModelValidator.commentModel); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	serializer := CommentSerializer{c, commentModelValidator.commentModel}
	c.JSON(http.StatusCreated, gin.H{"comment": serializer.Response()})
}

func ArticleCommentDelete(c *gin.Context) {
	slug := c.Param("slug")
	if _, err := FindOneArticle(&ArticleModel{Slug: slug}); err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}

	id64, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("comment", errors.New("not found")))
		return
	}
	id := uint(id64)
	commentModel, err := FindOneComment(&CommentModel{Model: gorm.Model{ID: id}})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("comment", errors.New("not found")))
		return
	}
	myUserModel := c.MustGet("my_user_model").(users.UserModel)
	articleUserModel := GetArticleUserModel(myUserModel)
	if commentModel.AuthorID != articleUserModel.ID {
		c.JSON(http.StatusForbidden, common.NewError("comment", errors.New("forbidden")))
		return
	}
	if err := DeleteCommentByID(id); err != nil {
		c.JSON(http.StatusUnprocessableEntity, common.NewError("database", err))
		return
	}
	c.Status(http.StatusNoContent)
}

func ArticleCommentList(c *gin.Context) {
	slug := c.Param("slug")
	articleModel, err := FindOneArticle(&ArticleModel{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("article", errors.New("not found")))
		return
	}
	err = articleModel.getComments()
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("comments", errors.New("Database error")))
		return
	}
	serializer := CommentsSerializer{c, articleModel.Comments}
	c.JSON(http.StatusOK, gin.H{"comments": serializer.Response()})
}

func TagList(c *gin.Context) {
	tagModels, err := getAllTags()
	if err != nil {
		c.JSON(http.StatusNotFound, common.NewError("articles", errors.New("Invalid param")))
		return
	}
	serializer := TagsSerializer{c, tagModels}
	c.JSON(http.StatusOK, gin.H{"tags": serializer.Response()})
}
