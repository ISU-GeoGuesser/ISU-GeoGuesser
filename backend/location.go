package main

import (
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

	lat, lon := 0.0, 0.0
	if gpsLat, gpsLon, ok := m.GPS(); ok {
		lat, lon = gpsLat, gpsLon
		response["latitude"] = lat
		response["longitude"] = lon
	}

	// _, err = db.Exec(
	// 	"INSERT INTO locations (filename, latitude, longitude) VALUES ($1, $2, $3)",
	// 	file.Filename, lat, lon,
	// )
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save location"})
	// 	return
	// }

	c.JSON(http.StatusOK, response)
}
