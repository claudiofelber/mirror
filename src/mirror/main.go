package main

import (
	"bufio"
	"code.google.com/p/go.crypto/ssh"
	"fmt"
	flags "github.com/jessevdk/go-flags"
	//"github.com/pkg/sftp"
	"github.com/claudiofelber/sftp"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const ProgramVersion = "1.0.0"

type options struct {
	Simulate       bool     `short:"s" long:"simulate" description:"Shows what files would be copied or deleted but does not actually do it"`
	Exclude        []string `short:"x" long:"exclude" description:"Source path to exclude from mirroring; removed from remote directory"`
	Ignore         []string `short:"i" long:"ignore" description:"Source path to exclude from mirroring; not removed from remote directory"`
	LocalPath      string
	RemoteHost     string
	RemotePort     int16
	RemoteUser     string
	RemotePassword string
	RemotePath     string
}

type fileInfo struct {
	path string
	info os.FileInfo
}

type filter struct {
	segment bool
	pattern *regexp.Regexp
}

type fileInfoSlice []fileInfo

func (slice fileInfoSlice) Len() int {
	return len(slice)
}

func (slice fileInfoSlice) Less(a, b int) bool {
	return slice[a].path < slice[b].path
}

func (slice fileInfoSlice) Swap(a, b int) {
	slice[a], slice[b] = slice[b], slice[a]
}

type passwordProvider string

func (p passwordProvider) Password(_ string) (string, error) {
	return string(p), nil
}

func main() {
	options := parseOptions()
	excludePaths := buildPathPatterns(options.Exclude)
	ignorePaths := buildPathPatterns(options.Ignore)
	localExcludes := append(excludePaths, ignorePaths...)

	fmt.Println("Analyzing local files")
	localFiles := getLocalFiles(options.LocalPath, localExcludes)

	if len(options.RemotePassword) == 0 {
		options.RemotePassword = readPassword()
	}

	client := connectToHost(options.RemoteHost, options.RemoteUser, options.RemotePassword, options.RemotePort)
	defer client.Close()

	fmt.Println("Analyzing remote files")
	remoteFiles := getRemoteFiles(client, options.RemotePath, ignorePaths)

	deletedFiles := getDeletedFiles(localFiles, remoteFiles)
	newFiles := getNewFiles(localFiles, remoteFiles)
	updatedFiles := getUpdatedFiles(localFiles, remoteFiles)

	deletedSuccessful, _ := deleteFiles(client, options.RemotePath, deletedFiles, options.Simulate)
	createdSuccessful, _ := copyFiles(client, options.LocalPath, options.RemotePath, newFiles, options.Simulate, false)
	updatedSuccessful, _ := copyFiles(client, options.LocalPath, options.RemotePath, updatedFiles, options.Simulate, true)

	if !options.Simulate {
		fmt.Printf("%d/%d created, %d/%d updated, %d/%d deleted\n",
			createdSuccessful, len(newFiles),
			updatedSuccessful, len(updatedFiles),
			deletedSuccessful, len(deletedFiles))
	}
}

func parseOptions() (options options) {
	parser := flags.NewParser(&options, flags.HelpFlag|flags.PassDoubleDash)
	parser.Usage = "[OPTIONS] localPath user[:password]@remoteHost[:port]/path"

	args, err := parser.Parse()
	if err != nil {
		if err.(*flags.Error).Type == flags.ErrHelp {
			printUsage(parser)
		} else {
			printError(err)
		}
		os.Exit(1)
	} else if len(args) != 2 {
		printUsage(parser)
		os.Exit(1)
	}

	re := regexp.MustCompile(`^([^\s:@]+)(?::([^\s:@]+))?@([^\s:/]+)(?::(\d+)?)?(\S*)?`)
	matches := re.FindStringSubmatch(args[1])
	if matches == nil {
		printError("Invalid remote path specification")
		os.Exit(1)
	} else {
		options.LocalPath, _ = filepath.Abs(args[0])
		options.RemoteUser = matches[1]
		options.RemotePassword = matches[2]
		options.RemoteHost = matches[3]
		options.RemotePort = 22
		options.RemotePath = path.Clean(matches[5])

		if matches[4] != "" {
			if port, err := strconv.ParseInt(matches[4], 10, 16); err != nil {
				printError("Invalid remote port " + matches[4])
				os.Exit(1)
			} else {
				options.RemotePort = int16(port)
			}
		}
	}

	return
}

func printUsage(parser *flags.Parser) {
	fmt.Printf("SFTP mirror %s, (c) 2013 Perron2 GmbH, Claudio Felber, All Rights Reserved\n\n", ProgramVersion)
	parser.WriteHelp(os.Stdout)
}

func printError(err interface{}) {
	fmt.Fprintln(os.Stderr, "Error:", err)
}

func buildPathPatterns(patterns []string) []filter {
	regexps := make([]filter, 0, 10)
	const placeholder = "\x01HOLD\x01"
	for _, pattern := range patterns {
		pattern = strings.Replace(pattern, "*", placeholder, -1)
		pattern = strings.Replace(pattern, `\`, `/`, -1)
		segment := true
		if len(pattern) > 0 && pattern[0] == '/' {
			segment = false
			pattern = pattern[1:]
		}
		pattern = regexp.QuoteMeta(pattern)
		pattern = strings.Replace(pattern, placeholder+placeholder, ".*?", -1)
		pattern = "^" + strings.Replace(pattern, placeholder, "[^/]*?", -1) + "$"
		re := regexp.MustCompile(pattern)
		regexps = append(regexps, filter{segment, re})
	}
	return regexps
}

func getLocalFiles(localPath string, filters []filter) []fileInfo {
	if stat, err := os.Stat(localPath); err != nil {
		printError(localPath + " does not exist")
		os.Exit(1)
	} else if !stat.IsDir() {
		printError(localPath + " is not a directory")
		os.Exit(1)
	}

	files := make([]fileInfo, 0, 10)
	localPathLength := len(localPath)
	filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		path = strings.Replace(strings.TrimLeft(path[localPathLength:], "/\\"), `\`, `/`, -1)
		if len(path) > 0 {
			files = append(files, fileInfo{path, info})
		}
		return nil
	})

	files = getFilteredPaths(files, filters)
	sort.Sort(fileInfoSlice(fileInfoSlice(files)))
	return files
}

func getRemoteFiles(client *sftp.Client, remotePath string, filters []filter) []fileInfo {
	if stat, err := client.Lstat(remotePath); err != nil {
		printError(remotePath + " does not exist")
		os.Exit(1)
	} else if !stat.IsDir() {
		printError(remotePath + " is not a directory")
		os.Exit(1)
	}

	walker := client.Walk(remotePath)
	files := make([]fileInfo, 0, 10)
	remotePathLength := 0
	if remotePath != "." {
		remotePathLength = len(remotePath)
	}

	for walker.Step() {
		path := walker.Path()
		path = strings.TrimLeft(path[remotePathLength:], "/")
		if len(path) > 0 {
			files = append(files, fileInfo{path, walker.Stat()})
		}
	}

	files = getFilteredPaths(files, filters)
	sort.Sort(fileInfoSlice(fileInfoSlice(files)))
	return files
}

func getFilteredPaths(files []fileInfo, filters []filter) []fileInfo {
	filtered := make([]fileInfo, 0, len(files))
	var excludeDir string
	for _, file := range files {
		if len(excludeDir) > 0 && strings.HasPrefix(file.path, excludeDir) {
			continue
		} else {
			include := true
			for _, filter := range filters {
				if filter.segment && filter.pattern.MatchString(file.info.Name()) ||
					!filter.segment && filter.pattern.MatchString(file.path) {
					if file.info.IsDir() {
						excludeDir = file.path
					}
					include = false
					break
				}
			}
			if include {
				filtered = append(filtered, file)
			}
		}
	}
	return filtered
}

func readPassword() (password string) {
	fmt.Print("Password: ")
	reader := bufio.NewReader(os.Stdin)
	if input, err := reader.ReadString('\n'); err != nil {
		printError("Cannot read password")
		os.Exit(1)
	} else {
		password = strings.TrimRight(input, "\n\r")
	}
	return
}

func connectToHost(host, user, password string, port int16) *sftp.Client {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.ClientAuth{
			ssh.ClientAuthPassword(passwordProvider(password)),
		},
	}

	hostAndPort := fmt.Sprintf("%s:%d", host, port)
	if conn, err := ssh.Dial("tcp", hostAndPort, config); err != nil {
		printError("Cannot connect to " + hostAndPort + "\n(" + err.Error() + ")")
		os.Exit(1)
		return nil
	} else {
		client, err := sftp.NewClient(conn)
		if err != nil {
			printError("Cannot connect to " + hostAndPort + "\n(" + err.Error() + ")")
			os.Exit(1)
			return nil
		} else {
			return client
		}
	}
}

func getDeletedFiles(localFiles, remoteFiles []fileInfo) []fileInfo {
	files := make([]fileInfo, 0, 10)
	for _, file := range remoteFiles {
		index := sort.Search(len(localFiles), func(index int) bool {
			return localFiles[index].path >= file.path
		})
		if len(localFiles) <= index || localFiles[index].path != file.path {
			files = append(files, file)
		} else if localFiles[index].info.IsDir() && !remoteFiles[index].info.IsDir() ||
			!localFiles[index].info.IsDir() && remoteFiles[index].info.IsDir() {
			// Delete remote file if local equivalent is a directory
			// Delete remote directory if local equivalend is a file
			files = append(files, file)
		}
	}
	return files
}

func getNewFiles(localFiles, remoteFiles []fileInfo) []fileInfo {
	files := make([]fileInfo, 0, 10)
	for _, file := range localFiles {
		index := sort.Search(len(remoteFiles), func(index int) bool {
			return remoteFiles[index].path >= file.path
		})
		if len(remoteFiles) <= index || remoteFiles[index].path != file.path {
			files = append(files, file)
		}
	}
	return files
}

func getUpdatedFiles(localFiles, remoteFiles []fileInfo) []fileInfo {
	files := make([]fileInfo, 0, 10)
	for _, file := range localFiles {
		index := sort.Search(len(remoteFiles), func(index int) bool {
			return remoteFiles[index].path >= file.path
		})
		if len(remoteFiles) > index && remoteFiles[index].path == file.path {
			if !file.info.IsDir() && file.info.ModTime().After(remoteFiles[index].info.ModTime()) {
				files = append(files, file)
			}
		}
	}
	return files
}

func deleteFiles(client *sftp.Client, path string, files []fileInfo, simulate bool) (successful, failed int) {
	if len(files) > 0 {
		fmt.Println("Deleting remote files")
		for i := len(files) - 1; i >= 0; i-- {
			if simulate {
				fmt.Println("  " + files[i].path)
			} else {
				fmt.Print("  " + files[i].path)
				if err := client.Remove(client.Join(path, files[i].path)); err != nil {
					failed++
					fmt.Println(" FAILED")
				} else {
					successful++
					fmt.Println()
				}
			}
		}
	}
	return
}

func copyFiles(client *sftp.Client, localPath, remotePath string, files []fileInfo, simulate, update bool) (successful, failed int) {
	if len(files) > 0 {
		if update {
			fmt.Println("Updating modified files")
		} else {
			fmt.Println("Creating new files")
		}
		for _, file := range files {
			if simulate {
				fmt.Println("  " + file.path)
			} else {
				fmt.Print("  " + file.path)

				if file.info.IsDir() {
					if err := client.Mkdir(client.Join(remotePath, file.path)); err == nil {
						successful++
						fmt.Println()
						continue
					}
				} else {
					path := client.Join(localPath, file.path)
					buffer, err := ioutil.ReadFile(path)
					if err == nil {
						path = client.Join(remotePath, file.path)
						file, err := client.Create(path)
						if err == nil {
							defer file.Close()
							_, err := file.Write(buffer)
							if err == nil {
								successful++
								fmt.Println()
								continue
							}
						}
					}
				}

				failed++
				fmt.Println(" FAILED")
			}
		}
	}
	return
}
