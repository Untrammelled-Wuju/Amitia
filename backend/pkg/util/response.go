// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/pkg/comment/response"
)

func SuccessResponse(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": data,
	})
}

func SuccessMsgResponse(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  msg,
		"data": data,
	})
}

func ErrorResponse(c *gin.Context, code int, msg string, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code": code,
		"msg":  msg,
		"data": data,
	})
}
