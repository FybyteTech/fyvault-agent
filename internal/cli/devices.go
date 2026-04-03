package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List devices in the current organization",
	RunE:  runDevicesList,
}

var devicesRegisterCmd = &cobra.Command{
	Use:   "devices:register",
	Short: "Register a new device (interactive)",
	RunE:  runDevicesRegister,
}

var devicesAssignCmd = &cobra.Command{
	Use:   "devices:assign [device-name] [secret-name]",
	Short: "Assign a secret to a device",
	Args:  cobra.ExactArgs(2),
	RunE:  runDevicesAssign,
}

type deviceItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Hostname    string `json:"hostname"`
	Status      string `json:"status"`
	LastSeen    string `json:"lastSeen"`
	SecretCount int    `json:"secretCount"`
}

func runDevicesList(_ *cobra.Command, _ []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	resp, err := apiRequest("GET", "/orgs/"+oid+"/devices", nil)
	if err != nil {
		return err
	}

	var devices []deviceItem
	if err := json.Unmarshal(resp.Data, &devices); err != nil {
		return fmt.Errorf("failed to parse devices: %w", err)
	}

	if format == "json" {
		out, _ := json.MarshalIndent(devices, "", "  ")
		fmt.Println(string(out))
		return nil
	}

	headers := []string{"NAME", "HOSTNAME", "STATUS", "LAST SEEN", "SECRETS"}
	rows := make([][]string, len(devices))
	for i, d := range devices {
		status := d.Status
		switch status {
		case "online":
			status = green(status)
		case "offline":
			status = red(status)
		default:
			status = yellow(status)
		}
		rows[i] = []string{
			d.Name,
			d.Hostname,
			status,
			d.LastSeen,
			fmt.Sprintf("%d", d.SecretCount),
		}
	}

	printTable(headers, rows)
	fmt.Printf("\n%s devices in organization\n", bold(fmt.Sprintf("%d", len(devices))))
	return nil
}

func runDevicesRegister(_ *cobra.Command, _ []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	fmt.Println(bold("Register a new device"))
	fmt.Println()

	name := promptInput("Device name: ")
	if name == "" {
		return fmt.Errorf("device name is required")
	}

	hostname := promptInput("Hostname (optional): ")

	body := map[string]string{
		"name": name,
	}
	if hostname != "" {
		body["hostname"] = hostname
	}

	resp, err := apiRequest("POST", "/orgs/"+oid+"/devices", body)
	if err != nil {
		return err
	}

	var created struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Data, &created); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Device '%s' registered (ID: %s)", bold(created.Name), created.ID))
	if created.Token != "" {
		fmt.Println()
		fmt.Println(yellow("Device token (save this — it won't be shown again):"))
		fmt.Println(bold(created.Token))
	}
	return nil
}

func runDevicesAssign(_ *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}
	oid := getOrgID(creds)
	if oid == "" {
		return fmt.Errorf("no organization selected. Run 'fyvault use <org-id>'")
	}

	deviceName := args[0]
	secretName := args[1]

	// Find device by name
	device, err := findDeviceByName(oid, deviceName)
	if err != nil {
		return err
	}

	// Find secret by name
	secret, err := findSecretByName(oid, secretName)
	if err != nil {
		return err
	}

	body := map[string]string{
		"secretId": secret.ID,
	}
	_, err = apiRequest("POST", "/orgs/"+oid+"/devices/"+device.ID+"/secrets", body)
	if err != nil {
		return err
	}

	printSuccess(fmt.Sprintf("Secret '%s' assigned to device '%s'", bold(secretName), bold(deviceName)))
	return nil
}

func findDeviceByName(orgID, name string) (*deviceItem, error) {
	resp, err := apiRequest("GET", "/orgs/"+orgID+"/devices", nil)
	if err != nil {
		return nil, err
	}

	var devices []deviceItem
	if err := json.Unmarshal(resp.Data, &devices); err != nil {
		return nil, fmt.Errorf("failed to parse devices: %w", err)
	}

	for _, d := range devices {
		if d.Name == name {
			return &d, nil
		}
	}

	return nil, fmt.Errorf("device '%s' not found", name)
}
