package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/S7evinK/issues-to-go/pkg/gh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

const configName = ".issues-to-go"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "issues-to-go",
	Example: `You need to set an environment variable GITHUB_TOKEN with a personal access token in it. After the first run this token can also be put in the generated config file.

Download all issues associated with the repository "S7evinK/issues-to-go" to a folder "./issues":
	GITHUB_TOKEN=mysecrettoken issues-to-go -r S7evinK/issues-to-go

Download all issues to a specific folder "output":
	issues-to-go -r S7evinK/issues-to-go -o ./output`,
	Short: "Downloads issues from Github for offline usage",
	Long: `issues-to-go downloads issues from Github for offline usage.
The default output format is Markdown. The issues are downloaded to a specified folder and to separate folders for open and closed issues.

After the first run a config file (.issues-to-go.yaml) will be created, subsequent runs from the same directory will use this file to determine the issues to download (if any).
`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		repo := viper.GetString("repo")
		since, err := time.Parse(time.RFC3339, viper.GetString("lastIssueTime"))
		if err != nil {
			since = time.Unix(0, 0)
		}

		cl, err := gh.New(
			gh.Output(viper.GetString("output")),
			gh.All(viper.GetBool("all")),
			gh.Count(viper.GetInt("count")),
			gh.UTC(viper.GetBool("utc")),
			gh.Since(viper.GetString("lastIssueTime")),
			gh.Repo(repo),
			gh.Token(viper.GetString("GITHUB_TOKEN")),
			gh.Milestones(viper.GetBool("milestones")),
		)
		if err != nil {
			log.Fatal("Unable to create new github client: ", err)
		}

		log.Printf("Getting new and updated issues/comments from %s since %v\n", repo, since.UTC())
		if err := cl.FetchIssues(); err != nil && err != gh.ErrNoIssues {
			log.Fatal("Unable to fetch issues: ", err)
		}

		// update lastIssueTime
		viper.Set("lastIssueTime", time.Now().UTC().Format(time.RFC3339))
		if err := viper.WriteConfigAs(configName + ".yaml"); err != nil {
			log.Fatal(fmt.Errorf("error writing to file: %v", err))
		}

		//cmd.Help()
		//fmt.Println("Couldn't determine repository. Make sure it's in the format USER/REPOSITORY")

	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .issues-to-go.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().StringP("repo", "r", "", "Repository to download (eg: S7evinK/issues-to-go)")
	rootCmd.Flags().StringP("output", "o", "./issues", "Output folder to download the issues to")
	rootCmd.Flags().Bool("utc", false, "Use UTC for dates. Defaults to false")
	rootCmd.Flags().IntP("count", "c", 100, "Sets the amount of issues/comments to fetch at once")
	rootCmd.Flags().Bool("all", false, "Get open and closed issues. By default only open issues will be downloaded")
	rootCmd.Flags().Bool("milestones", false, "Create a separate folder with issues linked to milestones.")

	_ = viper.BindPFlags(rootCmd.Flags())

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in the current working directory with name ".issues-to-go" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigName(configName)
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
	}
}
