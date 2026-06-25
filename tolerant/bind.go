// Copyright 2025 Gin Team. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package tolerant 提供 Gin 框架的容错参数绑定扩展。
// 与原生 ShouldBindQuery / ShouldBindUri 不同，容错绑定在单个字段解析失败时
// 不会整体失败，而是将该字段设为对应类型零值，继续解析其余字段，并收集所有错误明细返回。
//
// 支持两种错误类型：
//   - parse: 字段类型转换失败（如 "abc" 转为 int），字段设为零值
//   - validate: 字段解析成功但 binding 校验不通过（如 gte=1 但值为 0），字段保留已解析值
package tolerant

import (
	"encoding"
	"errors"
	"fmt"
	"mime/multipart"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/gin-gonic/gin/codec/json"
	"github.com/gin-gonic/gin/internal/bytesconv"
	"github.com/go-playground/validator/v10"
)

// 错误类型常量
const (
	// ErrTypeParse 表示字段类型解析失败（如 "abc" 转为 int），字段将被设为零值。
	ErrTypeParse = "parse"
	// ErrTypeValidate 表示字段解析成功但 binding 校验不通过（如 required 字段缺失），字段保留已解析值。
	ErrTypeValidate = "validate"
)

// FieldBindErr 表示单个字段的绑定错误。
type FieldBindErr struct {
	// Field 是出错的字段名（对应 form 或 uri 标签值）。
	Field string `json:"field"`
	// Reason 是错误原因描述。
	Reason string `json:"reason"`
	// Type 是错误类型：parse（解析失败）或 validate（校验失败）。
	Type string `json:"type"`
}

// FieldBindErrors 是 FieldBindErr 的切片，实现 error 接口。
type FieldBindErrors []FieldBindErr

// Error 实现 error 接口，返回所有错误的汇总信息。
func (e FieldBindErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	var b strings.Builder
	for i := range e {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString("[")
		b.WriteString(e[i].Type)
		b.WriteString("] ")
		b.WriteString(e[i].Field)
		b.WriteString(": ")
		b.WriteString(e[i].Reason)
	}
	return b.String()
}

// HasErrors 返回是否有绑定错误。
func (e FieldBindErrors) HasErrors() bool {
	return len(e) > 0
}

// ShouldBindQueryTolerant 对 Query 参数进行容错绑定。
// 单个字段类型转换失败时，该字段设为零值，其余合法字段正常解析。
// 返回所有出错字段的明细；若全部成功则返回空切片。
func ShouldBindQueryTolerant(c *gin.Context, obj any) FieldBindErrors {
	values := c.Request.URL.Query()
	return mapFormTolerant(obj, values, "form")
}

// ShouldBindURITolerant 对 URI 路径参数进行容错绑定。
// 单个字段类型转换失败时，该字段设为零值，其余合法字段正常解析。
// 返回所有出错字段的明细；若全部成功则返回空切片。
func ShouldBindURITolerant(c *gin.Context, obj any) FieldBindErrors {
	m := make(map[string][]string, len(c.Params))
	for _, v := range c.Params {
		m[v.Key] = []string{v.Value}
	}
	return mapFormTolerant(obj, m, "uri")
}

// ============================================================================
// 核心容错映射逻辑
// ============================================================================

// tolerantContext 在容错映射过程中传递上下文信息。
type tolerantContext struct {
	errs      FieldBindErrors
	parseErrs map[string]bool // 记录已发生 parse 错误的字段名，用于跳过冗余的校验错误
}

func (tc *tolerantContext) addParseErr(fieldName string, reason string) {
	tc.parseErrs[fieldName] = true
	tc.errs = append(tc.errs, FieldBindErr{
		Field:  fieldName,
		Reason: reason,
		Type:   ErrTypeParse,
	})
}

func mapFormTolerant(ptr any, form map[string][]string, tag string) FieldBindErrors {
	ptrVal := reflect.ValueOf(ptr)
	var pointed any
	if ptrVal.Kind() == reflect.Ptr {
		ptrVal = ptrVal.Elem()
		pointed = ptrVal.Interface()
	}
	if ptrVal.Kind() == reflect.Map && ptrVal.Type().Key().Kind() == reflect.String {
		if pointed != nil {
			ptr = pointed
		}
		return setFormMapTolerant(ptr, form)
	}

	tc := &tolerantContext{
		parseErrs: make(map[string]bool),
	}
	mappingByPtrTolerant(ptr, formSource(form), tag, tc)

	// 容错映射后仍然执行校验，收集校验错误
	// 跳过已发生 parse 错误的字段，避免重复报告
	if binding.Validator != nil {
		if verr := binding.Validator.ValidateStruct(ptr); verr != nil {
			collectValidationErrors(verr, ptr, tag, tc)
		}
	}

	return tc.errs
}

func setFormMapTolerant(ptr any, form map[string][]string) FieldBindErrors {
	el := reflect.TypeOf(ptr).Elem()
	if el.Kind() == reflect.Slice {
		ptrMap, ok := ptr.(map[string][]string)
		if !ok {
			return FieldBindErrors{{Field: "map", Reason: binding.ErrConvertMapStringSlice.Error(), Type: ErrTypeParse}}
		}
		for k, v := range form {
			ptrMap[k] = v
		}
		return nil
	}
	ptrMap, ok := ptr.(map[string]string)
	if !ok {
		return FieldBindErrors{{Field: "map", Reason: binding.ErrConvertToMapString.Error(), Type: ErrTypeParse}}
	}
	for k, v := range form {
		ptrMap[k] = v[len(v)-1]
	}
	return nil
}

func collectValidationErrors(verr error, obj any, tag string, tc *tolerantContext) {
	valErrs, ok := verr.(validator.ValidationErrors)
	if !ok {
		tc.errs = append(tc.errs, FieldBindErr{
			Field:  "validation",
			Reason: verr.Error(),
			Type:   ErrTypeValidate,
		})
		return
	}
	for _, fe := range valErrs {
		fieldName := fe.Field()
		if tagName := resolveTagName(obj, fe.StructField(), tag); tagName != "" {
			fieldName = tagName
		}
		// 跳过已发生 parse 错误的字段，避免 parse+validate 双重报错
		if tc.parseErrs[fieldName] {
			continue
		}
		tc.errs = append(tc.errs, FieldBindErr{
			Field:  fieldName,
			Reason: fmt.Sprintf("%s (rule: %s)", fe.Tag(), fe.Param()),
			Type:   ErrTypeValidate,
		})
	}
}

// resolveTagName 通过反射查找 struct 字段对应的 tag 值。
func resolveTagName(obj any, structField string, tag string) string {
	v := reflect.ValueOf(obj)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	t := v.Type()
	for i := range t.NumField() {
		sf := t.Field(i)
		if sf.Name == structField {
			tagVal := sf.Tag.Get(tag)
			if name, _, _ := strings.Cut(tagVal, ","); name != "" {
				return name
			}
			return sf.Name
		}
		if sf.Anonymous {
			if nested := resolveTagName(v.Field(i).Addr().Interface(), structField, tag); nested != "" {
				return nested
			}
		}
	}
	return ""
}

// ============================================================================
// 反射映射引擎（修改自 gin/binding/form_mapping.go）
// ============================================================================

// setter 尝试在结构体字段遍历过程中设置值
type setter interface {
	TrySet(value reflect.Value, field reflect.StructField, key string, opt setOptions) (isSet bool, err error)
}

type formSource map[string][]string

var _ setter = formSource(nil)

func (form formSource) TrySet(value reflect.Value, field reflect.StructField, tagValue string, opt setOptions) (isSet bool, err error) {
	return setByForm(value, field, form, tagValue, opt)
}

var emptyField = reflect.StructField{}

func mappingByPtrTolerant(ptr any, setter setter, tag string, tc *tolerantContext) {
	tolerantMapping(reflect.ValueOf(ptr), emptyField, setter, tag, tc)
}

// tolerantMapping 是容错版本的 mapping 函数。
// 与 gin 原版不同，当单个字段解析失败时，它会将该字段设为零值并继续处理其余字段。
func tolerantMapping(value reflect.Value, field reflect.StructField, setter setter, tag string, tc *tolerantContext) (bool, error) {
	if field.Tag.Get(tag) == "-" {
		return false, nil
	}

	vKind := value.Kind()

	if vKind == reflect.Ptr {
		var isNew bool
		vPtr := value
		if value.IsNil() {
			isNew = true
			vPtr = reflect.New(value.Type().Elem())
		}
		isSet, err := tolerantMapping(vPtr.Elem(), field, setter, tag, tc)
		if err != nil {
			return false, nil
		}
		if isNew && isSet {
			value.Set(vPtr)
		}
		return isSet, nil
	}

	if vKind != reflect.Struct || !field.Anonymous {
		ok, err := tryToSetValue(value, field, setter, tag)
		if err != nil {
			// 容错核心：设置零值 + 收集 parse 错误
			fieldName := resolveFieldName(field, tag)
			value.Set(reflect.Zero(value.Type()))
			tc.addParseErr(fieldName, err.Error())
			// 继续处理后续字段，不返回 error
			if vKind == reflect.Struct {
				tValue := value.Type()
				var isSet bool
				for i := range value.NumField() {
					sf := tValue.Field(i)
					if sf.PkgPath != "" && !sf.Anonymous {
						continue
					}
					ok, _ := tolerantMapping(value.Field(i), sf, setter, tag, tc)
					isSet = isSet || ok
				}
				return isSet, nil
			}
			return false, nil
		}
		if ok {
			return true, nil
		}
	}

	if vKind == reflect.Struct {
		tValue := value.Type()
		var isSet bool
		for i := range value.NumField() {
			sf := tValue.Field(i)
			if sf.PkgPath != "" && !sf.Anonymous {
				continue
			}
			ok, _ := tolerantMapping(value.Field(i), sf, setter, tag, tc)
			isSet = isSet || ok
		}
		return isSet, nil
	}
	return false, nil
}

// resolveFieldName 获取字段的错误报告名称，优先使用 tag 值。
func resolveFieldName(field reflect.StructField, tag string) string {
	tagVal := field.Tag.Get(tag)
	if name, _, _ := strings.Cut(tagVal, ","); name != "" {
		return name
	}
	return field.Name
}

// ============================================================================
// 以下函数复制自 gin/binding/form_mapping.go，保持逻辑一致
// ============================================================================

type setOptions struct {
	isDefaultExists bool
	defaultValue    string
	parser          string
}

func tryToSetValue(value reflect.Value, field reflect.StructField, setter setter, tag string) (bool, error) {
	tagValue := field.Tag.Get(tag)
	tagValue, opts := head(tagValue, ",")

	if tagValue == "" {
		tagValue = field.Name
	}
	if tagValue == "" {
		return false, nil
	}

	var setOpt setOptions
	var opt string
	for len(opts) > 0 {
		opt, opts = head(opts, ",")
		if k, v := head(opt, "="); k == "default" {
			setOpt.isDefaultExists = true
			setOpt.defaultValue = v
			if field.Type.Kind() == reflect.Slice || field.Type.Kind() == reflect.Array {
				cfTag := field.Tag.Get("collection_format")
				if cfTag == "" || cfTag == "multi" || cfTag == "csv" {
					setOpt.defaultValue = strings.ReplaceAll(v, ";", ",")
				}
			}
		} else if k, v = head(opt, "="); k == "parser" {
			setOpt.parser = v
		}
	}

	return setter.TrySet(value, field, tagValue, setOpt)
}

// BindUnmarshaler 用于自定义类型解析 form/query 参数的接口。
type BindUnmarshaler interface {
	UnmarshalParam(param string) error
}

func trySetCustom(val string, value reflect.Value) (isSet bool, err error) {
	switch v := value.Addr().Interface().(type) {
	case BindUnmarshaler:
		return true, v.UnmarshalParam(val)
	}
	return false, nil
}

func trySetUsingParser(val string, value reflect.Value, parser string) (isSet bool, err error) {
	switch parser {
	case "encoding.TextUnmarshaler":
		v, ok := value.Addr().Interface().(encoding.TextUnmarshaler)
		if !ok {
			return false, nil
		}
		return true, v.UnmarshalText([]byte(val))
	}
	return false, nil
}

func trySplit(vs []string, field reflect.StructField) ([]string, error) {
	cfTag := field.Tag.Get("collection_format")
	if cfTag == "" || cfTag == "multi" {
		return vs, nil
	}
	var sep string
	switch cfTag {
	case "csv":
		sep = ","
	case "ssv":
		sep = " "
	case "tsv":
		sep = "\t"
	case "pipes":
		sep = "|"
	default:
		return vs, fmt.Errorf("%s is not supported in the collection_format. (multi, csv, ssv, tsv, pipes)", cfTag)
	}
	totalLength := 0
	for _, v := range vs {
		totalLength += strings.Count(v, sep) + 1
	}
	newVs := make([]string, 0, totalLength)
	for _, v := range vs {
		newVs = append(newVs, strings.Split(v, sep)...)
	}
	return newVs, nil
}

func setByForm(value reflect.Value, field reflect.StructField, form map[string][]string, tagValue string, opt setOptions) (isSet bool, err error) {
	vs, ok := form[tagValue]
	if !ok && !opt.isDefaultExists {
		return false, nil
	}

	switch value.Kind() {
	case reflect.Slice:
		if len(vs) == 0 {
			if !opt.isDefaultExists {
				return false, nil
			}
			vs = []string{opt.defaultValue}
			cfTag := field.Tag.Get("collection_format")
			if cfTag == "" || cfTag == "multi" {
				vs = strings.Split(opt.defaultValue, ",")
			}
		}
		if ok, err = trySetUsingParser(vs[0], value, opt.parser); ok {
			return ok, err
		} else if ok, err = trySetCustom(vs[0], value); ok {
			return ok, err
		}
		if vs, err = trySplit(vs, field); err != nil {
			return false, err
		}
		return true, setSlice(vs, value, field, opt)
	case reflect.Array:
		if len(vs) == 0 {
			if !opt.isDefaultExists {
				return false, nil
			}
			vs = []string{opt.defaultValue}
			cfTag := field.Tag.Get("collection_format")
			if cfTag == "" || cfTag == "multi" {
				vs = strings.Split(opt.defaultValue, ",")
			}
		}
		if ok, err = trySetUsingParser(vs[0], value, opt.parser); ok {
			return ok, err
		} else if ok, err = trySetCustom(vs[0], value); ok {
			return ok, err
		}
		if vs, err = trySplit(vs, field); err != nil {
			return false, err
		}
		if len(vs) != value.Len() {
			return false, fmt.Errorf("%q is not valid value for %s", vs, value.Type().String())
		}
		return true, setArray(vs, value, field, opt)
	default:
		var val string
		if !ok || len(vs) == 0 || (len(vs) > 0 && vs[0] == "") {
			val = opt.defaultValue
		} else if len(vs) > 0 {
			val = vs[0]
		}
		if ok, err = trySetUsingParser(val, value, opt.parser); ok {
			return ok, err
		} else if ok, err = trySetCustom(val, value); ok {
			return ok, err
		}
		return true, setWithProperType(val, value, field, opt)
	}
}

func setWithProperType(val string, value reflect.Value, field reflect.StructField, opt setOptions) error {
	if ok, err := trySetUsingParser(val, value, opt.parser); ok {
		return err
	} else if ok, err = trySetCustom(val, value); ok {
		return err
	}

	if value.Kind() != reflect.String {
		val = strings.TrimSpace(val)
	}

	switch value.Kind() {
	case reflect.Int:
		return setIntField(val, 0, value)
	case reflect.Int8:
		return setIntField(val, 8, value)
	case reflect.Int16:
		return setIntField(val, 16, value)
	case reflect.Int32:
		return setIntField(val, 32, value)
	case reflect.Int64:
		switch value.Interface().(type) {
		case time.Duration:
			return setTimeDuration(val, value)
		}
		return setIntField(val, 64, value)
	case reflect.Uint:
		return setUintField(val, 0, value)
	case reflect.Uint8:
		return setUintField(val, 8, value)
	case reflect.Uint16:
		return setUintField(val, 16, value)
	case reflect.Uint32:
		return setUintField(val, 32, value)
	case reflect.Uint64:
		return setUintField(val, 64, value)
	case reflect.Bool:
		return setBoolField(val, value)
	case reflect.Float32:
		return setFloatField(val, 32, value)
	case reflect.Float64:
		return setFloatField(val, 64, value)
	case reflect.String:
		value.SetString(val)
	case reflect.Struct:
		switch value.Interface().(type) {
		case time.Time:
			return setTimeField(val, field, value)
		case multipart.FileHeader:
			return nil
		}
		return json.API.Unmarshal(bytesconv.StringToBytes(val), value.Addr().Interface())
	case reflect.Map:
		return json.API.Unmarshal(bytesconv.StringToBytes(val), value.Addr().Interface())
	case reflect.Ptr:
		if !value.Elem().IsValid() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return setWithProperType(val, value.Elem(), field, opt)
	default:
		return errUnknownType
	}
	return nil
}

var errUnknownType = errors.New("unknown type")

func setIntField(val string, bitSize int, field reflect.Value) error {
	if val == "" {
		val = "0"
	}
	intVal, err := strconv.ParseInt(val, 10, bitSize)
	if err == nil {
		field.SetInt(intVal)
	}
	return err
}

func setUintField(val string, bitSize int, field reflect.Value) error {
	if val == "" {
		val = "0"
	}
	uintVal, err := strconv.ParseUint(val, 10, bitSize)
	if err == nil {
		field.SetUint(uintVal)
	}
	return err
}

func setBoolField(val string, field reflect.Value) error {
	if val == "" {
		val = "false"
	}
	boolVal, err := strconv.ParseBool(val)
	if err == nil {
		field.SetBool(boolVal)
	}
	return err
}

func setFloatField(val string, bitSize int, field reflect.Value) error {
	if val == "" {
		val = "0.0"
	}
	floatVal, err := strconv.ParseFloat(val, bitSize)
	if err == nil {
		field.SetFloat(floatVal)
	}
	return err
}

func setTimeField(val string, structField reflect.StructField, value reflect.Value) error {
	timeFormat := structField.Tag.Get("time_format")
	if timeFormat == "" {
		timeFormat = time.RFC3339
	}
	if val == "" {
		value.Set(reflect.ValueOf(time.Time{}))
		return nil
	}
	switch tf := strings.ToLower(timeFormat); tf {
	case "unix", "unixmilli", "unixmicro", "unixnano":
		tv, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		var t time.Time
		switch tf {
		case "unix":
			t = time.Unix(tv, 0)
		case "unixmilli":
			t = time.UnixMilli(tv)
		case "unixmicro":
			t = time.UnixMicro(tv)
		default:
			t = time.Unix(0, tv)
		}
		value.Set(reflect.ValueOf(t))
		return nil
	}
	l := time.Local
	if isUTC, _ := strconv.ParseBool(structField.Tag.Get("time_utc")); isUTC {
		l = time.UTC
	}
	if locTag := structField.Tag.Get("time_location"); locTag != "" {
		loc, err := time.LoadLocation(locTag)
		if err != nil {
			return err
		}
		l = loc
	}
	t, err := time.ParseInLocation(timeFormat, val, l)
	if err != nil {
		return err
	}
	value.Set(reflect.ValueOf(t))
	return nil
}

func setArray(vals []string, value reflect.Value, field reflect.StructField, opt setOptions) error {
	for i, s := range vals {
		err := setWithProperType(s, value.Index(i), field, opt)
		if err != nil {
			return err
		}
	}
	return nil
}

func setSlice(vals []string, value reflect.Value, field reflect.StructField, opt setOptions) error {
	slice := reflect.MakeSlice(value.Type(), len(vals), len(vals))
	err := setArray(vals, slice, field, opt)
	if err != nil {
		return err
	}
	value.Set(slice)
	return nil
}

func setTimeDuration(val string, value reflect.Value) error {
	if val == "" {
		val = "0"
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return err
	}
	value.Set(reflect.ValueOf(d))
	return nil
}

func head(str, sep string) (head string, tail string) {
	head, tail, _ = strings.Cut(str, sep)
	return head, tail
}