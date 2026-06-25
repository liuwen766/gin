// Copyright 2025 Gin Team. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tolerant_test

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/tolerant"
)

// ExampleShouldBindQueryTolerant 演示 Query 参数容错绑定。
// 当 page 参数传入非法值时，该字段设为零值（parse 错误），
// 其余字段正常解析；当字段解析成功但校验不通过时（validate 错误），
// 字段保留已解析值。接口返回 400 并附带所有错误明细及类型。
func ExampleShouldBindQueryTolerant() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	type ListQuery struct {
		Page     int    `form:"page"      binding:"gte=1"`
		PageSize int    `form:"page_size" binding:"gte=1,lte=100"`
		Keyword  string `form:"keyword"`
	}

	router.GET("/api/items", func(c *gin.Context) {
		var q ListQuery
		errs := tolerant.ShouldBindQueryTolerant(c, &q)
		if errs.HasErrors() {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "部分参数解析失败",
				"errors":  errs, // 每个错误包含 field、reason、type（parse/validate）
				"data":    q,    // 合法字段已正常赋值
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"page": q.Page, "page_size": q.PageSize, "keyword": q.Keyword})
	})

	_ = router
}

// ExampleShouldBindURITolerant 演示 URI 路径参数容错绑定。
// 当 user_id 路径参数传入非法值时，该字段设为零值（parse 错误），
// 其余字段正常解析；校验失败（validate 错误）时字段保留已解析值。
// 收集所有错误明细，按类型区分返回。
func ExampleShouldBindURITolerant() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	type UserAction struct {
		UserID int    `uri:"user_id" binding:"gte=1"`
		Action string `uri:"action"`
	}

	router.GET("/api/users/:user_id/:action", func(c *gin.Context) {
		var u UserAction
		errs := tolerant.ShouldBindURITolerant(c, &u)
		if errs.HasErrors() {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "部分参数解析失败",
				"errors":  errs, // 每个错误包含 field、reason、type（parse/validate）
				"data":    u,    // 合法字段已正常赋值
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"user_id": u.UserID, "action": u.Action})
	})

	_ = router
}

// ExampleFieldBindErrors 演示错误结构和错误类型的用法。
func ExampleFieldBindErrors() {
	// parse 错误：字段类型转换失败，设为零值
	// validate 错误：字段校验不通过，保留已解析值
	errs := tolerant.FieldBindErrors{
		{Field: "age", Reason: "invalid syntax", Type: tolerant.ErrTypeParse},
		{Field: "page", Reason: "gte (rule: 1)", Type: tolerant.ErrTypeValidate},
	}

	if errs.HasErrors() {
		_ = errs.Error()
		// 输出: [parse] age: invalid syntax; [validate] page: gte (rule: 1)
	}
}
