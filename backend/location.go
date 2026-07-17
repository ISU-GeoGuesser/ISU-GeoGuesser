package main

import (
	"net/http"

	gometadata "github.com/FlavioCFOliveira/GoMetadata"
	"github.com/gin-gonic/gin"

	"isu-geoguesser/config"
	// db "isu-geoguesser/database"
)

func uploadLocation(c *gin.Context) {
	name := c.PostForm("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Location name is required"})
		return
	}

	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image received: " + err.Error()})
		return
	}

	dst := config.IMAGE_DIR + file.Filename
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
		"name":     name,
		"filename": file.Filename,
		"size":     file.Size,
	}

	lat, lon := 0.0, 0.0
	if gpsLat, gpsLon, ok := m.GPS(); ok {
		lat, lon = gpsLat, gpsLon
		response["latitude"] = lat
		response["longitude"] = lon
	}

	// _, err = db.DB.Exec(
	// 	db.INSERT_LOCATION,
	// 	file.Filename, name, lat, lon,
	// )
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save location"})
	// 	return
	// }

	c.JSON(http.StatusOK, response)
}
