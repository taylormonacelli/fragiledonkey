package cleanup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/taylormonacelli/fragiledonkey/query"
)

func RunCleanup(olderThan string) {
	duration, err := parseDuration(olderThan)
	if err != nil {
		fmt.Println("Error parsing duration:", err)
		return
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		fmt.Println("Error loading config:", err)
		return
	}

	client := ec2.NewFromConfig(cfg, func(o *ec2.Options) {
		o.Region = "us-west-2"
	})

	jsonData, err := os.ReadFile("northflier-amis.json")
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	var amis []query.AMI
	err = json.Unmarshal(jsonData, &amis)
	if err != nil {
		fmt.Println("Error unmarshaling JSON:", err)
		return
	}

	now := time.Now()
	var imagesToDelete []string
	var snapshotsToDelete []string

	for _, ami := range amis {
		if now.Sub(ami.CreationDate) > duration {
			imagesToDelete = append(imagesToDelete, ami.ID)
			snapshotsToDelete = append(snapshotsToDelete, ami.Snapshots...)
		}
	}

	if len(imagesToDelete) == 0 && len(snapshotsToDelete) == 0 {
		fmt.Println("No AMIs or snapshots to delete.")
		return
	}

	fmt.Println("AMIs to be deleted:")
	for _, imageID := range imagesToDelete {
		fmt.Println("-", imageID)
	}

	fmt.Println("Snapshots to be deleted:")
	for _, snapshotID := range snapshotsToDelete {
		fmt.Println("-", snapshotID)
	}

	fmt.Print("Do you want to proceed with the deletion? (y/n): ")
	var confirm string
	fmt.Scanln(&confirm)

	if confirm != "y" {
		fmt.Println("Aborting deletion.")
		return
	}

	for _, imageID := range imagesToDelete {
		input := &ec2.DeregisterImageInput{
			ImageId: aws.String(imageID),
		}

		_, err := client.DeregisterImage(context.Background(), input)
		if err != nil {
			fmt.Printf("Error deregistering AMI %s: %v\n", imageID, err)
			continue
		}

		fmt.Printf("Deregistered AMI: %s\n", imageID)
	}

	for _, snapshotID := range snapshotsToDelete {
		input := &ec2.DeleteSnapshotInput{
			SnapshotId: aws.String(snapshotID),
		}

		_, err := client.DeleteSnapshot(context.Background(), input)
		if err != nil {
			fmt.Printf("Error deleting snapshot %s: %v\n", snapshotID, err)
			continue
		}

		fmt.Printf("Deleted snapshot: %s\n", snapshotID)
	}

	fmt.Println("Cleanup completed.")
}

func parseDuration(duration string) (time.Duration, error) {
	unitMap := map[string]time.Duration{
		"s": time.Second,
		"m": time.Minute,
		"h": time.Hour,
		"d": 24 * time.Hour,
		"M": 30 * 24 * time.Hour,
		"y": 365 * 24 * time.Hour,
	}

	value := duration[:len(duration)-1]
	unit := duration[len(duration)-1:]

	if _, ok := unitMap[unit]; !ok {
		return 0, fmt.Errorf("invalid duration unit: %s", unit)
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", value)
	}

	return time.Duration(intValue) * unitMap[unit], nil
}