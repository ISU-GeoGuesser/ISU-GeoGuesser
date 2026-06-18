package main

import (
	"log"
	"net/http"

	gometadata "github.com/FlavioCFOliveira/GoMetadata"
	"github.com/gin-gonic/gin"
)

func uploadLocation(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image received: " + err.Error()})
		return
	}

	dst := "./images/" + file.Filename
	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	m, err := gometadata.ReadFile(dst)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read metadata: " + err.Error()})
		return
	}

	response := gin.H{
		"message":  "Image uploaded successfully",
		"filename": file.Filename,
		"size":     file.Size,
	}

	if lat, lon, ok := m.GPS(); ok {
		response["latitude"] = lat
		response["longitude"] = lon
	}

	// TODO: save dst and location in database

	c.JSON(http.StatusOK, response)
}

func main() {
	r := gin.Default()

	r.POST("/locations", uploadLocation)

	if err := r.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
