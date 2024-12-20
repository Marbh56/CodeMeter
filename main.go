package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {

	repl()
}

func repl() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Printf(">> ")
		scanner.Scan()
		line := scanner.Text()
		if line == "exit" {
			break
		}
		parts := strings.SplitN(line, " ", 2)
		cmd := parts[0]
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}
		switch cmd {
		case "scan":
			handleScanCommand(args)
		case "save":
			handleSaveCommand(args)
		default:
			fmt.Println("Unknown command")
		}
	}
}

type FileContributor struct {
	Name       string
	Commits    int
	LineCount  int
	LastCommit string
}

type DirStats struct {
	FileCount    int
	LineCount    int
	Contributors map[string]FileContributor
	IsGitRepo    bool
}

func getGitContributors(path string) (map[string]FileContributor, bool, error) {
	contributors := make(map[string]FileContributor)

	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	if err := cmd.Run(); err != nil {
		return contributors, false, nil
	}

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		cmd := exec.Command("git", "blame", "--line-porcelain", filepath.Base(filePath))
		cmd.Dir = filepath.Dir(filePath)
		output, err := cmd.Output()
		if err != nil {
			return nil
		}
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		currentAuthor := ""
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "author ") {
				currentAuthor = strings.TrimPrefix(line, "author ")
				contrib := contributors[currentAuthor]
				contrib.Name = currentAuthor
				contrib.LineCount++
				contributors[currentAuthor] = contrib
			}
		}
		return nil
	})
	if err != nil {
		return contributors, true, err
	}

	cmd = exec.Command("git", "shortlog", "-sn", "--all")
	cmd.Dir = path
	output, err := cmd.Output()
	if err != nil {
		return contributors, true, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			commitCount := 0
			fmt.Sscanf(fields[0], "%d", &commitCount)
			name := strings.Join(fields[1:], " ")
			if contrib, ok := contributors[name]; ok {
				contrib.Commits = commitCount
				contributors[name] = contrib
			}
		}
	}
	return contributors, true, nil
}

func scanDirectory(dirPath string) (DirStats, error) {
	stats := DirStats{
		Contributors: make(map[string]FileContributor),
	}

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		if strings.Contains(path, ".git") {
			return nil
		}

		stats.FileCount++
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %v", path, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			stats.LineCount++
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to scan file %s: %v", path, err)
		}
		return nil
	})

	contributors, isGitRepo, err := getGitContributors(dirPath)
	if err != nil {
		return stats, fmt.Errorf("error getting git contributors: %v", err)
	}
	stats.Contributors = contributors
	stats.IsGitRepo = isGitRepo

	return stats, nil
}

func handleScanCommand(args string) {
	dirPath := strings.TrimSpace(args)
	if dirPath == "" {
		dirPath = "."
	}

	stats, err := scanDirectory(dirPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Directory: %s\nFiles: %d\nTotal Lines: %d\n", dirPath, stats.FileCount, stats.LineCount)
}

func handleSaveCommand(args string) {
	parts := strings.SplitN(args, " ", 2)
	if len(parts) < 2 {
		fmt.Println("Usage: save <directory> <output-file>")
		return
	}
	dirPath := parts[0]
	outputFile := parts[1]

	if _, err := os.Stat(outputFile); err == nil {
		fmt.Printf("File %s already exists. Overwrite? (y/n): ", outputFile)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Operation cancelled")
			return
		}
	}

	stats, err := scanDirectory(dirPath)
	if err != nil {
		fmt.Printf("Error scanning directory: %v\n", err)
		return
	}
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	report := fmt.Sprintf("Directory Scan Report\n"+"Generated: %s\n\n"+
		"Directory: %s\n"+
		"Total Files: %d\n"+
		"Total Lines: %d\n",
		time.Now().Format(time.RFC1123),
		dirPath,
		stats.FileCount,
		stats.LineCount)

	if stats.IsGitRepo {
		report += "\nGit Contributors:\n"
		for _, contrib := range stats.Contributors {
			report += fmt.Sprintf("- %s:\n"+
				"	Commits: %d\n"+
				"	Lines: %d\n",
				contrib.Name,
				contrib.Commits,
				contrib.LineCount)
		}
	}

	if _, err := file.WriteString(report); err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}
	fmt.Printf("Report saved to %s\n", outputFile)
}
