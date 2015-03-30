package main

import (
	"console"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const ProgramVersion = "1.1.2"

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
	path    string
	info    os.FileInfo
	deleted bool
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

	if options.RemotePassword == "" {
		fmt.Print("Enter remote password: ")
		options.RemotePassword = console.ReadPassword()
		fmt.Println()
	}

	fmt.Println("Connecting to", options.RemoteHost)
	client := connectToHost(options.RemoteHost, options.RemoteUser, options.RemotePassword, options.RemotePort)
	defer client.Close()

	fmt.Println("Analyzing local files")
	localFiles := getLocalFiles(options.LocalPath, localExcludes)

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
	fmt.Printf("SFTP mirror %s, (c) 2013–1015 Perron2 GmbH, Claudio Felber, All Rights Reserved\n\n", ProgramVersion)
	parser.WriteHelp(os.Stdout)
	fmt.Println()
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
			files = append(files, fileInfo{path, info, false})
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
			files = append(files, fileInfo{path, walker.Stat(), false})
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

func connectToHost(host, user, password string, port int16) *sftp.Client {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
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
	for remoteIndex, file := range remoteFiles {
		index := sort.Search(len(localFiles), func(index int) bool {
			return localFiles[index].path >= file.path
		})
		if len(localFiles) <= index || localFiles[index].path != file.path {
			files = append(files, file)
		} else if localFiles[index].info.IsDir() && !file.info.IsDir() ||
			!localFiles[index].info.IsDir() && file.info.IsDir() {
			// Delete remote file if local equivalent is a directory
			// Delete remote directory if local equivalent is a file
			files = append(files, file)
			remoteFiles[remoteIndex].deleted = true
			//file.deleted = true

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
		if len(remoteFiles) <= index || remoteFiles[index].path != file.path || remoteFiles[index].deleted {
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
			dateChanged := file.info.ModTime().Unix() > remoteFiles[index].info.ModTime().Unix()
			sizeChanged := file.info.Size() != remoteFiles[index].info.Size()
			if !file.info.IsDir() && !remoteFiles[index].deleted && (dateChanged || sizeChanged) {
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
					console.SetTextColor(console.LIGHTRED)
					fmt.Print(" FAILED")
					console.ResetColor()
					fmt.Println()
				} else {
					successful++
					console.SetTextColor(console.LIGHTGREEN)
					fmt.Print(" OK")
					console.ResetColor()
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
						console.SetTextColor(console.LIGHTGREEN)
						fmt.Print(" OK")
						console.ResetColor()
						fmt.Println()
						continue
					}
				} else {
					path := client.Join(localPath, file.path)
					localFile, err := os.Open(path)
					if err == nil {
						stat, _ := localFile.Stat()
						path = client.Join(remotePath, file.path)
						remoteFile, err := client.Create(path)
						if err == nil {
							written, err := copyFile(localFile, remoteFile, stat.Size())
							if err == nil && written == stat.Size() {
								client.Chtimes(path, file.info.ModTime(), file.info.ModTime())
								successful++
								console.SetTextColor(console.LIGHTGREEN)
								fmt.Print(" OK")
								console.ResetColor()
								fmt.Println()
								localFile.Close()
								remoteFile.Close()
								continue
							}
							remoteFile.Close()
						}
						localFile.Close()
					}
				}

				failed++
				console.SetTextColor(console.LIGHTRED)
				fmt.Print(" FAILED")
				console.ResetColor()
				fmt.Println()
			}
		}
	}
	return
}

func copyFile(source io.Reader, dest io.Writer, length int64) (written int64, err error) {
	buf := make([]byte, 32*1024)
	showPercentage := length > int64(len(buf))
	percentage := ""
	if showPercentage {
		percentage = " 0%"
	}
	fmt.Print(percentage)
	for {
		nr, er := source.Read(buf)
		if nr > 0 {
			nw, ew := dest.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			err = er
			break
		}
		if showPercentage {
			fmt.Print(strings.Repeat("\b", len(percentage)))
			percentage = " " + strconv.Itoa(int(float64(written*100)/float64(length)+0.5)) + "%"
			fmt.Print(percentage)
		}
	}
	if showPercentage {
		fmt.Print(strings.Repeat("\b", len(percentage)))
		fmt.Print(strings.Repeat(" ", len(percentage)))
		fmt.Print(strings.Repeat("\b", len(percentage)))
	}
	return written, err
}
