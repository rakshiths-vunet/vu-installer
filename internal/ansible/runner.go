package ansible

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"vu-installer/internal/state"

	log "github.com/sirupsen/logrus"
)

type Runner struct {
	InventoryPath string
	PlaybookPath  string
	UpdateTasks   func(nodeName string, tasks []state.Task)
}

func (r *Runner) GenerateInventory(nodeName, ip, user, keyPath, version, ip1, ip2, ip3, vmname1, vmname2, vmname3 string) error {
	log.WithFields(log.Fields{"node": nodeName, "ip": ip, "version": version}).Info("Generating Ansible inventory")

	// Create unique inventory filename
	inventoryFile := fmt.Sprintf("inventory_%s.ini", nodeName)

	// Ensure directory exists (if path contains dir)
	if err := os.MkdirAll(filepath.Dir(inventoryFile), 0755); err != nil {
		log.WithError(err).Error("Failed to create inventory directory")
		return err
	}

	var content string
	if version == "3node3.0" {
		content = fmt.Sprintf("[%s]\n", nodeName)
		content += fmt.Sprintf("%s ansible_host=%s ansible_user=%s ansible_ssh_private_key_file=%s node_hostname=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'\n", vmname1, ip1, user, keyPath, vmname1)
		content += fmt.Sprintf("%s ansible_host=%s ansible_user=%s ansible_ssh_private_key_file=%s node_hostname=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'\n", vmname2, ip2, user, keyPath, vmname2)
		content += fmt.Sprintf("%s ansible_host=%s ansible_user=%s ansible_ssh_private_key_file=%s node_hostname=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'\n", vmname3, ip3, user, keyPath, vmname3)
	} else {
		content = fmt.Sprintf("[%s]\n%s ansible_host=%s ansible_user=%s ansible_ssh_private_key_file=%s node_hostname=%s ansible_ssh_common_args='-o StrictHostKeyChecking=no'\n",
			nodeName, nodeName, ip, user, keyPath, nodeName)
	}

	err := os.WriteFile(inventoryFile, []byte(content), 0644)
	if err != nil {
		log.WithError(err).Error("Failed to write inventory file")
	} else {
		log.Info("Inventory file generated successfully")
	}
	return err
}

func (r *Runner) Run(nodeName, ip, version, startAtTask, ip1, ip2, ip3, vmname1, vmname2, vmname3 string) error {
	log.WithFields(log.Fields{"node": nodeName, "ip": ip, "version": version, "start_at": startAtTask}).Info("Starting Ansible playbook execution")
	// Pass Go variables to Ansible as Extra Vars
	extraVars := fmt.Sprintf("private_ip=%s node_name=%s version=%s ip1=%s ip2=%s ip3=%s vmname1=%s vmname2=%s vmname3=%s", ip, nodeName, version, ip1, ip2, ip3, vmname1, vmname2, vmname3)

	// Open log file for Ansible output
	logFile, err := os.Create(fmt.Sprintf("ansible_%s.log", nodeName))
	if err != nil {
		log.WithError(err).Error("Failed to create Ansible log file")
		return err
	}
	defer logFile.Close()

	// Use unique inventory file
	inventoryFile := fmt.Sprintf("inventory_%s.ini", nodeName)

	args := []string{
		"-i", inventoryFile,
		r.PlaybookPath,
		"--extra-vars", extraVars,
	}

	if startAtTask != "" {
		args = append(args, "--start-at-task", startAtTask)
	}

	cmd := exec.Command("ansible-playbook", args...)

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.WithError(err).Error("Failed to get stdout pipe")
		return err
	}
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)

	log.Info("Executing Ansible playbook")

	// Start the command
	if err := cmd.Start(); err != nil {
		log.WithError(err).Error("Failed to start Ansible playbook")
		return err
	}

	// Read stdout line by line
	scanner := bufio.NewScanner(stdout)
	tasks := []state.Task{}
	taskRegex := regexp.MustCompile(`^TASK \[(.*)\]`)
	resultRegex := regexp.MustCompile(`^(ok|failed|changed|skipped|unreachable|fatal):`)

	for scanner.Scan() {
		line := scanner.Text()
		// Write to stdout and log file
		fmt.Println(line)
		logFile.WriteString(line + "\n")

		// Parse for tasks
		if matches := taskRegex.FindStringSubmatch(line); matches != nil {
			taskName := matches[1]
			tasks = append(tasks, state.Task{Name: taskName, Status: "running"})
			if r.UpdateTasks != nil {
				r.UpdateTasks(nodeName, tasks)
			}
		} else if matches := resultRegex.FindStringSubmatch(line); matches != nil {
			result := matches[1]
			if len(tasks) > 0 {
				lastTask := &tasks[len(tasks)-1]
				switch result {
				case "ok", "changed":
					lastTask.Status = "success"
				case "failed", "fatal":
					lastTask.Status = "failed"
				case "skipped":
					lastTask.Status = "skipped"
				case "unreachable":
					lastTask.Status = "unreachable"
				}
				if r.UpdateTasks != nil {
					r.UpdateTasks(nodeName, tasks)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.WithError(err).Error("Error reading stdout")
	}

	// Wait for command to finish
	err = cmd.Wait()
	if err != nil {
		log.WithError(err).Error("Ansible playbook execution failed")
	} else {
		log.Info("Ansible playbook executed successfully")
	}
	return err
}

func (r *Runner) MonitorInstallLog(nodeName, version string) error {
	log.WithField("node", nodeName).Info("Monitoring install_log.txt for completion")
	completionMessage := "Installation completed"
	timeout := 60 // minutes
	installPath := fmt.Sprintf("/home/vunet/vuSmartMaps_offline_NG-%s/install_log.txt", version)
	for i := 0; i < timeout; i++ {
		cmd := exec.Command("ansible", "-i", r.InventoryPath, nodeName, "-m", "shell", "-a", fmt.Sprintf("grep -q '%s' %s && echo found || echo not", completionMessage, installPath))
		output, err := cmd.Output()
		if err != nil {
			log.WithError(err).Error("Failed to check install log")
			return err
		}
		if strings.Contains(string(output), "found") {
			log.Info("Installation completed")
			return nil
		}
		time.Sleep(60 * time.Second)
	}
	return fmt.Errorf("timeout waiting for installation completion")
}

func (r *Runner) Cleanup(nodeName string) {
	log.WithField("node", nodeName).Info("Cleaning up inventory file")
	inventoryFile := fmt.Sprintf("inventory_%s.ini", nodeName)
	err := os.Remove(inventoryFile)
	if err != nil {
		log.WithError(err).Warn("Failed to remove inventory file")
	} else {
		log.Info("Inventory file cleaned up")
	}
}
