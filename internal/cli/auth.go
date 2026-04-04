package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/fybyte/fyvault-agent/internal/config"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to FyVault",
	Long:  "Authenticate with the FyVault API and save credentials locally.",
	RunE:  runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of FyVault",
	Long:  "Remove locally saved credentials.",
	RunE:  runLogout,
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently logged-in user",
	RunE:  runWhoami,
}

func runLogin(_ *cobra.Command, _ []string) error {
	fmt.Println(bold("FyVault Login"))
	fmt.Println()

	// 1. API URL (production default; use http://localhost:4000/api/v1 for local cloud)
	urlInput := promptInput(fmt.Sprintf("API URL %s: ", dim(fmt.Sprintf("(default: %s)", config.DefaultCloudAPIURL))))
	base := config.DefaultCloudAPIURL
	if urlInput != "" {
		base = urlInput
	}

	// 2. Email
	email := promptInput("Email: ")
	if email == "" {
		return fmt.Errorf("email is required")
	}

	// 3. Password
	password := promptPassword("Password: ")
	if password == "" {
		return fmt.Errorf("password is required")
	}

	// 4. POST /auth/login
	loginBody := map[string]string{
		"email":    email,
		"password": password,
	}

	resp, err := apiRequestUnauth(base, "POST", "/auth/login", loginBody)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	// Parse login response
	var loginData struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		RequiresTotp bool   `json:"requiresTotp"`
		TempToken    string `json:"tempToken"`
	}
	if err := json.Unmarshal(resp.Data, &loginData); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	accessToken := loginData.AccessToken
	refreshToken := loginData.RefreshToken

	// 5. Handle TOTP if required
	if loginData.RequiresTotp {
		fmt.Println()
		fmt.Println(yellow("Two-factor authentication required."))
		totpCode := promptInput("TOTP code: ")
		if totpCode == "" {
			return fmt.Errorf("TOTP code is required")
		}

		totpBody := map[string]string{
			"tempToken": loginData.TempToken,
			"totpCode":  totpCode,
		}
		totpResp, totpErr := apiRequestUnauth(base, "POST", "/auth/totp/login", totpBody)
		if totpErr != nil {
			return fmt.Errorf("TOTP verification failed: %w", totpErr)
		}

		var totpData struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		}
		if err := json.Unmarshal(totpResp.Data, &totpData); err != nil {
			return fmt.Errorf("failed to parse TOTP response: %w", err)
		}
		accessToken = totpData.AccessToken
		refreshToken = totpData.RefreshToken
	}

	if accessToken == "" {
		return fmt.Errorf("login succeeded but no access token received")
	}

	// 6. Fetch user's orgs
	orgsResp, err := apiRequestWithToken(base, accessToken, "GET", "/auth/me/orgs", nil)

	var selectedOrgID string
	if err == nil && orgsResp != nil {
		var orgs []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		}
		if jsonErr := json.Unmarshal(orgsResp.Data, &orgs); jsonErr == nil && len(orgs) > 0 {
			if len(orgs) == 1 {
				selectedOrgID = orgs[0].ID
				fmt.Printf("\nUsing organization: %s\n", bold(orgs[0].Name))
			} else {
				// Let user pick
				fmt.Println()
				options := make([]string, len(orgs))
				for i, o := range orgs {
					options[i] = fmt.Sprintf("%s (%s)", o.Name, dim(o.ID))
				}
				idx, _ := promptSelect("Select an organization:", options)
				selectedOrgID = orgs[idx].ID
			}
		}
	}

	// 7. Save credentials
	creds := &Credentials{
		APIUrl:       base,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		OrgID:        selectedOrgID,
		Email:        email,
	}
	if err := saveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println()
	printSuccess(fmt.Sprintf("Logged in as %s", bold(email)))
	if selectedOrgID != "" {
		fmt.Printf("  Organization: %s\n", selectedOrgID)
	}
	fmt.Printf("  Credentials saved to %s\n", dim(credentialsPath()))

	return nil
}

func runLogout(_ *cobra.Command, _ []string) error {
	if err := deleteCredentials(); err != nil {
		// Ignore error if file doesn't exist
		fmt.Println("Already logged out.")
		return nil
	}
	printSuccess("Logged out. Credentials removed.")
	return nil
}

func runWhoami(_ *cobra.Command, _ []string) error {
	creds, err := loadCredentials()
	if err != nil {
		return err
	}

	fmt.Printf("Email:   %s\n", bold(creds.Email))
	fmt.Printf("Org:     %s\n", creds.OrgID)
	fmt.Printf("API URL: %s\n", dim(creds.APIUrl))
	return nil
}
