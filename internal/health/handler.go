package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type statusResponse struct {
	Status string `json:"status"`
}

// Handler reports that the process is running.
// It does not check the database.
func Handler(c *gin.Context) {
	c.JSON(http.StatusOK, statusResponse{Status: "ok"})
}
