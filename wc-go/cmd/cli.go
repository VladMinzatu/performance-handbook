package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/VladMinzatu/performance-handbook/wc-go/processing"
	"github.com/spf13/cobra"
)

func Run() {
	var processorType string
	var rootCmd = &cobra.Command{
		Use:   "wc-go [file]",
		Short: "A word count utility",
		Long:  `A simple word count utility that can process files with different processors.`,
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filePath := ""
			if len(args) > 0 {
				filePath = args[0]
			}

			processor := &processing.InputProcessor{ProcessorType: processorType, FilePath: filePath}

			lineProcessors := []processing.LineProcessor{
				&processing.LineCountProcessor{},
				&processing.WordCountProcessor{},
				&processing.CharacterCountProcessor{},
			}

			err := processor.Run(lineProcessors)
			if err != nil {
				log.Fatalf("error processing file: %v", err)
				os.Exit(1)
			}

			for _, lineProcessor := range lineProcessors {
				fmt.Printf("%d\t", lineProcessor.Count())
			}
			fmt.Printf("%s\n", filePath)
		},
	}

	rootCmd.Flags().StringVarP(&processorType, "processor", "p", "not provided", "Processor to use for word counting")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
