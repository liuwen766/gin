// Copyright 2025 Gin Team. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tolerant

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ============================================================================
// 测试用结构体
// ============================================================================

type QueryParams struct {
	Name   string  `form:"name"`
	Age    int     `form:"age"`
	Score  float64 `form:"score"`
	Active bool    `form:"active"`
}

type URIParams struct {
	ID     int    `uri:"id"`
	Name   string `uri:"name"`
	Status string `uri:"status"`
}

type MixedParams struct {
	ID    uint   `form:"id"`
	Count uint8  `form:"count"`
	Flag  uint16 `form:"flag"`
	Big   uint32 `form:"big"`
	Huge  uint64 `form:"huge"`
}

type IntOverflowParams struct {
	Small int8  `form:"small"`
	Big   int16 `form:"big"`
	Word  int32 `form:"word"`
	Long  int64 `form:"long"`
}

type FloatParams struct {
	F32 float32 `form:"f32"`
	F64 float64 `form:"f64"`
}

type DefaultParams struct {
	Name string `form:"name,default=default_name"`
	Age  int    `form:"age,default=25"`
}

type BindingTagParams struct {
	Name  string `form:"name"  binding:"required"`
	Email string `form:"email" binding:"required,email"`
	Age   int    `form:"age"   binding:"gte=0,lte=150"`
}

type ParseAndValidateParams struct {
	Page     int    `form:"page"     binding:"gte=1"`
	PageSize int    `form:"page_size" binding:"gte=1,lte=100"`
	Keyword  string `form:"keyword"  binding:"required"`
}

// ============================================================================
// ShouldBindQueryTolerant 测试
// ============================================================================

func TestShouldBindQueryTolerant_AllValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=john&age=30&score=95.5&active=true", nil)

	var params QueryParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.False(t, errs.HasErrors(), "期望无错误，实际: %v", errs)
	assert.Equal(t, "john", params.Name)
	assert.Equal(t, 30, params.Age)
	assert.Equal(t, 95.5, params.Score)
	assert.Equal(t, true, params.Active)
}

func TestShouldBindQueryTolerant_SingleFieldError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=john&age=not_a_number&score=95.5&active=true", nil)

	var params QueryParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors(), "期望有错误")
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, "age", errs[0].Field)
	assert.Equal(t, ErrTypeParse, errs[0].Type)
	assert.Contains(t, errs[0].Reason, "invalid syntax")

	// 合法字段正常解析
	assert.Equal(t, "john", params.Name)
	assert.Equal(t, 95.5, params.Score)
	assert.Equal(t, true, params.Active)
	// 出错字段为零值
	assert.Equal(t, 0, params.Age)
}

func TestShouldBindQueryTolerant_MultipleFieldErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=john&age=abc&score=xyz&active=yes", nil)

	var params QueryParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 3, len(errs))

	// 验证每个错误字段和类型
	fields := make(map[string]string)
	for _, e := range errs {
		fields[e.Field] = e.Type
	}
	assert.Equal(t, ErrTypeParse, fields["age"])
	assert.Equal(t, ErrTypeParse, fields["score"])
	assert.Equal(t, ErrTypeParse, fields["active"])

	// 合法字段正常解析
	assert.Equal(t, "john", params.Name)
	// 出错字段为零值
	assert.Equal(t, 0, params.Age)
	assert.Equal(t, 0.0, params.Score)
	assert.Equal(t, false, params.Active)
}

func TestShouldBindQueryTolerant_MissingParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=john", nil)

	var params QueryParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.False(t, errs.HasErrors())
	assert.Equal(t, "john", params.Name)
	assert.Equal(t, 0, params.Age)
	assert.Equal(t, 0.0, params.Score)
	assert.Equal(t, false, params.Active)
}

func TestShouldBindQueryTolerant_EmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	var params QueryParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.False(t, errs.HasErrors())
	assert.Equal(t, "", params.Name)
	assert.Equal(t, 0, params.Age)
	assert.Equal(t, 0.0, params.Score)
	assert.Equal(t, false, params.Active)
}

// ============================================================================
// ShouldBindURITolerant 测试
// ============================================================================

func TestShouldBindURITolerant_AllValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/42/john/active", nil)
	c.Params = gin.Params{
		{Key: "id", Value: "42"},
		{Key: "name", Value: "john"},
		{Key: "status", Value: "active"},
	}

	var params URIParams
	errs := ShouldBindURITolerant(c, &params)

	assert.False(t, errs.HasErrors(), "期望无错误，实际: %v", errs)
	assert.Equal(t, 42, params.ID)
	assert.Equal(t, "john", params.Name)
	assert.Equal(t, "active", params.Status)
}

func TestShouldBindURITolerant_SingleFieldError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/abc/john/active", nil)
	c.Params = gin.Params{
		{Key: "id", Value: "not_a_number"},
		{Key: "name", Value: "john"},
		{Key: "status", Value: "active"},
	}

	var params URIParams
	errs := ShouldBindURITolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, "id", errs[0].Field)
	assert.Equal(t, ErrTypeParse, errs[0].Type)
	assert.Contains(t, errs[0].Reason, "invalid syntax")

	assert.Equal(t, "john", params.Name)
	assert.Equal(t, "active", params.Status)
	assert.Equal(t, 0, params.ID)
}

func TestShouldBindURITolerant_MultipleFieldErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/abc/xxx/yyy", nil)
	c.Params = gin.Params{
		{Key: "id", Value: "not_int"},
		{Key: "name", Value: ""},
		{Key: "status", Value: ""},
	}

	var params URIParams
	errs := ShouldBindURITolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, "id", errs[0].Field)
	assert.Equal(t, ErrTypeParse, errs[0].Type)
}

func TestShouldBindURITolerant_MissingParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/user/42", nil)
	c.Params = gin.Params{
		{Key: "id", Value: "42"},
	}

	var params URIParams
	errs := ShouldBindURITolerant(c, &params)

	assert.False(t, errs.HasErrors())
	assert.Equal(t, 42, params.ID)
	assert.Equal(t, "", params.Name)
	assert.Equal(t, "", params.Status)
}

// ============================================================================
// 数值溢出测试
// ============================================================================

func TestShouldBindQueryTolerant_IntOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?small=128&big=40000&word=3000000000&long=99999999999999999999", nil)

	var params IntOverflowParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 4, len(errs))

	fields := make(map[string]string)
	for _, e := range errs {
		fields[e.Field] = e.Type
	}
	assert.Equal(t, ErrTypeParse, fields["small"])
	assert.Equal(t, ErrTypeParse, fields["big"])
	assert.Equal(t, ErrTypeParse, fields["word"])
	assert.Equal(t, ErrTypeParse, fields["long"])

	assert.Equal(t, int8(0), params.Small)
	assert.Equal(t, int16(0), params.Big)
	assert.Equal(t, int32(0), params.Word)
	assert.Equal(t, int64(0), params.Long)
}

func TestShouldBindQueryTolerant_UintOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?id=-1&count=-1&flag=-1&big=-1&huge=-1", nil)

	var params MixedParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 5, len(errs))
	for _, e := range errs {
		assert.Equal(t, ErrTypeParse, e.Type)
	}
	assert.Equal(t, uint(0), params.ID)
	assert.Equal(t, uint8(0), params.Count)
	assert.Equal(t, uint16(0), params.Flag)
	assert.Equal(t, uint32(0), params.Big)
	assert.Equal(t, uint64(0), params.Huge)
}

func TestShouldBindQueryTolerant_FloatOverflow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?f32=1e50&f64=1e500", nil)

	var params FloatParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 2, len(errs))
	for _, e := range errs {
		assert.Equal(t, ErrTypeParse, e.Type)
	}
}

// ============================================================================
// 混合场景测试
// ============================================================================

func TestShouldBindQueryTolerant_PartialError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=alice&age=25&score=bad&active=true", nil)

	var params QueryParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, "score", errs[0].Field)
	assert.Equal(t, ErrTypeParse, errs[0].Type)

	assert.Equal(t, "alice", params.Name)
	assert.Equal(t, 25, params.Age)
	assert.Equal(t, 0.0, params.Score)
	assert.Equal(t, true, params.Active)
}

func TestShouldBindQueryTolerant_DefaultValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	var params DefaultParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.False(t, errs.HasErrors())
	assert.Equal(t, "default_name", params.Name)
	assert.Equal(t, 25, params.Age)
}

func TestShouldBindQueryTolerant_DefaultWithInvalidValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?age=not_a_number", nil)

	var params DefaultParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())
	assert.Equal(t, 1, len(errs))
	assert.Equal(t, "age", errs[0].Field)
	assert.Equal(t, ErrTypeParse, errs[0].Type)
	assert.Equal(t, 0, params.Age)
}

// ============================================================================
// 校验标签兼容性测试（两处漏洞修复验证）
// ============================================================================

// TestShouldBindQueryTolerant_ValidationError_FieldKeepsValue
// 验证漏洞修复1：字段解析成功但 binding 校验不通过时，字段保留已解析值，错误纳入列表
func TestShouldBindQueryTolerant_ValidationError_FieldKeepsValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// page=0 解析成功，但 binding:"gte=1" 校验不通过
	// keyword 缺失，binding:"required" 校验不通过
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=0&page_size=20", nil)

	var params ParseAndValidateParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())

	// 验证错误类型：全部为 validate
	for _, e := range errs {
		assert.Equal(t, ErrTypeValidate, e.Type, "字段 %s 期望 validate 类型", e.Field)
	}

	// 验证字段保留已解析值（不是零值）
	assert.Equal(t, 0, params.Page)      // 解析值为 0，合法
	assert.Equal(t, 20, params.PageSize) // 解析值为 20，合法
	assert.Equal(t, "", params.Keyword)  // 缺失，空字符串
}

// TestShouldBindQueryTolerant_MixedParseAndValidate
// 验证：同时存在 parse 错误和 validate 错误时，parse 错误字段设为零值，
// validate 错误字段保留已解析值；parse 错误字段不重复产生 validate 错误
func TestShouldBindQueryTolerant_MixedParseAndValidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// page=abc 解析失败 → parse 错误，设为零值
	// page_size=20 解析成功且校验通过
	// keyword 缺失 → validate 错误（required）
	c.Request = httptest.NewRequest(http.MethodGet, "/?page=abc&page_size=20", nil)

	var params ParseAndValidateParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())

	// 分类检查错误
	var parseErrs, validateErrs FieldBindErrors
	for _, e := range errs {
		switch e.Type {
		case ErrTypeParse:
			parseErrs = append(parseErrs, e)
		case ErrTypeValidate:
			validateErrs = append(validateErrs, e)
		}
	}

	// page 解析失败：1 个 parse 错误
	assert.Equal(t, 1, len(parseErrs), "期望 1 个 parse 错误")
	assert.Equal(t, "page", parseErrs[0].Field)

	// keyword 缺失：1 个 validate 错误
	assert.Equal(t, 1, len(validateErrs), "期望 1 个 validate 错误")
	assert.Equal(t, "keyword", validateErrs[0].Field)

	// page 不应重复出现在 validate 错误中
	for _, e := range validateErrs {
		assert.NotEqual(t, "page", e.Field, "page 已有 parse 错误，不应再出现 validate 错误")
	}

	// page 解析失败 → 零值
	assert.Equal(t, 0, params.Page)
	// page_size 解析成功且校验通过 → 保留值
	assert.Equal(t, 20, params.PageSize)
	// keyword 缺失 → 空字符串
	assert.Equal(t, "", params.Keyword)
}

func TestShouldBindQueryTolerant_ValidationPass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=john&email=john@example.com&age=30", nil)

	var params BindingTagParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.False(t, errs.HasErrors(), "期望无校验错误，实际: %v", errs)
	assert.Equal(t, "john", params.Name)
	assert.Equal(t, "john@example.com", params.Email)
	assert.Equal(t, 30, params.Age)
}

// TestShouldBindQueryTolerant_ValidateOnly_ParseAllSuccess
// 验证：所有字段解析成功，但部分校验失败，字段保留已解析值
func TestShouldBindQueryTolerant_ValidateOnly_ParseAllSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	// 所有字段解析成功，但 email 格式错误，age 超出范围
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=john&email=not-an-email&age=200", nil)

	var params BindingTagParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.True(t, errs.HasErrors())

	allValidate := true
	for _, e := range errs {
		if e.Type != ErrTypeValidate {
			allValidate = false
		}
	}
	assert.True(t, allValidate, "期望所有错误均为 validate 类型")

	// 字段保留已解析值
	assert.Equal(t, "john", params.Name)
	assert.Equal(t, "not-an-email", params.Email)
	assert.Equal(t, 200, params.Age)
}

// ============================================================================
// Error() 输出格式测试
// ============================================================================

func TestFieldBindErrors_Error(t *testing.T) {
	errs := FieldBindErrors{
		{Field: "age", Reason: "invalid syntax", Type: ErrTypeParse},
		{Field: "score", Reason: "value out of range", Type: ErrTypeParse},
	}
	errStr := errs.Error()
	assert.Contains(t, errStr, "[parse] age: invalid syntax")
	assert.Contains(t, errStr, "[parse] score: value out of range")
	assert.Contains(t, errStr, "; ")
}

func TestFieldBindErrors_Error_MixedTypes(t *testing.T) {
	errs := FieldBindErrors{
		{Field: "page", Reason: "invalid syntax", Type: ErrTypeParse},
		{Field: "keyword", Reason: "required (rule: )", Type: ErrTypeValidate},
	}
	errStr := errs.Error()
	assert.Contains(t, errStr, "[parse] page: invalid syntax")
	assert.Contains(t, errStr, "[validate] keyword: required (rule: )")
}

func TestFieldBindErrors_Empty(t *testing.T) {
	var errs FieldBindErrors
	assert.False(t, errs.HasErrors())
	assert.Equal(t, "", errs.Error())
}

// ============================================================================
// 边界条件测试
// ============================================================================

func TestShouldBindQueryTolerant_NilStructPointer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=test", nil)

	assert.Panics(t, func() {
		var p *QueryParams = nil
		ShouldBindQueryTolerant(c, p)
	})
}

func TestShouldBindQueryTolerant_NonStructPointer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?value=123", nil)

	var val int
	errs := ShouldBindQueryTolerant(c, &val)
	assert.False(t, errs.HasErrors())
}

func TestShouldBindQueryTolerant_TagSkip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	type SkipParams struct {
		Name    string `form:"name"`
		Ignored string `form:"-"`
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?name=john&ignored=should_be_ignored", nil)

	var params SkipParams
	errs := ShouldBindQueryTolerant(c, &params)

	assert.False(t, errs.HasErrors())
	assert.Equal(t, "john", params.Name)
	assert.Equal(t, "", params.Ignored)
}

// ============================================================================
// 示例接口测试
// ============================================================================

func TestExample_QueryTolerantEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	type ExampleQuery struct {
		Page     int    `form:"page"     binding:"gte=1"`
		PageSize int    `form:"page_size" binding:"gte=1,lte=100"`
		Keyword  string `form:"keyword"`
	}

	router.GET("/api/items", func(c *gin.Context) {
		var q ExampleQuery
		errs := ShouldBindQueryTolerant(c, &q)
		if errs.HasErrors() {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "部分参数解析失败",
				"errors":  errs,
				"data":    q,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": q})
	})

	// 测试：部分字段解析失败（parse 错误）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items?page=abc&page_size=20&keyword=test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "部分参数解析失败")
	assert.Contains(t, w.Body.String(), "page")
	assert.Contains(t, w.Body.String(), "parse")
}

func TestExample_QueryTolerantEndpoint_ValidationOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	type ExampleQuery struct {
		Page     int    `form:"page"     binding:"gte=1"`
		PageSize int    `form:"page_size" binding:"gte=1,lte=100"`
		Keyword  string `form:"keyword"`
	}

	router.GET("/api/items", func(c *gin.Context) {
		var q ExampleQuery
		errs := ShouldBindQueryTolerant(c, &q)
		if errs.HasErrors() {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "部分参数校验失败",
				"errors":  errs,
				"data":    q,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": q})
	})

	// 测试：解析成功但校验失败（validate 错误），page=0 不满足 gte=1
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items?page=0&page_size=20&keyword=test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "validate")
	// page 字段保留解析值 0
	assert.Contains(t, w.Body.String(), "\"Page\":0")
}

func TestExample_URITolerantEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	type ExampleURI struct {
		UserID int    `uri:"user_id" binding:"gte=1"`
		Action string `uri:"action"`
	}

	router.GET("/api/users/:user_id/:action", func(c *gin.Context) {
		var u ExampleURI
		errs := ShouldBindURITolerant(c, &u)
		if errs.HasErrors() {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "部分参数解析失败",
				"errors":  errs,
				"data":    u,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": u})
	})

	// 测试：user_id 非法（parse 错误）
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users/abc/view", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "部分参数解析失败")
	assert.Contains(t, w.Body.String(), "user_id")
	assert.Contains(t, w.Body.String(), "parse")
}