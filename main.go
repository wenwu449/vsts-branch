package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type secrets struct {
	Username                 string `json:"username"`
	Password                 string `json:"password"`
	Instance                 string `json:"instance"`
	Collection               string `json:"collection"`
	Project                  string `json:"project"`
	Repo                     string `json:"repo"`
	MasterBranch             string `json:"masterBranch"`
	ReleaseBranchPrefix      string `json:"releaseBranchPrefix"`
	VersionPath              string `json:"versionPath"`
	DefinitionPathPrefix     string `json:"definitionPathPrefix"`
	DefinitionName           string `json:"definitionName"`
	OnboardBuildDefinitionID int    `json:"OnboardBuildDefinitionId"`
}

type ref struct {
	Name     string `json:"name"`
	ObjectID string `json:"objectId"`
	URL      string `json:"url"`
}

type refs struct {
	Value []ref `json:"value"`
	Count int   `json:"count"`
}

type branch struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
	NewObjectID string `json:"newObjectId"`
}

type commits struct {
	Count int `json:"count"`
	Value []struct {
		CommitID string `json:"commitId"`
		Author   struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Name  string    `json:"name"`
			Email string    `json:"email"`
			Date  time.Time `json:"date"`
		} `json:"committer"`
		Comment      string `json:"comment"`
		ChangeCounts struct {
			Add int `json:"Add"`
		} `json:"changeCounts"`
		URL       string `json:"url"`
		RemoteURL string `json:"remoteUrl"`
	} `json:"value"`
}

type refUpdate struct {
	Name        string `json:"name"`
	OldObjectID string `json:"oldObjectId"`
}

type item struct {
	Path string `json:"path"`
}

type newContent struct {
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

type change struct {
	ChangeType string     `json:"changeType"`
	Item       item       `json:"item"`
	NewContent newContent `json:"newContent"`
}

type commit struct {
	Comment string   `json:"comment"`
	Changes []change `json:"changes"`
}

type push struct {
	RefUpdates []refUpdate `json:"refUpdates"`
	Commits    []commit    `json:"commits"`
}

type version struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type root struct {
	Versions []version `xml:"versions>version"`
}

type definition struct {
	ID int `json:"id"`
}

type buildReq struct {
	Definition   definition `json:"definition"`
	SourceBranch string     `json:"sourceBranch"`
	Parameters   string     `json:"parameters"`
}

type definitions struct {
	Count int `json:"count"`
	Value []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
		URI  string `json:"uri"`
		Path string `json:"path"`
		Type string `json:"type"`
	} `json:"value"`
}

type builds struct {
	Count int `json:"count"`
	Value []struct {
		ID            int       `json:"id"`
		URL           string    `json:"url"`
		BuildNumber   string    `json:"buildNumber"`
		URI           string    `json:"uri"`
		SourceBranch  string    `json:"sourceBranch"`
		SourceVersion string    `json:"sourceVersion"`
		Status        string    `json:"status"`
		QueueTime     time.Time `json:"queueTime"`
		Priority      string    `json:"priority"`
		StartTime     time.Time `json:"startTime"`
		FinishTime    time.Time `json:"finishTime"`
		Reason        string    `json:"reason"`
		Result        string    `json:"result"`
		Parameters    string    `json:"parameters"`
		KeepForever   bool      `json:"keepForever"`
	} `json:"value"`
}

var secret = secrets{}

func getRelBranches(client *http.Client, relBranch string) refs {
	getBranchURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/refs/heads/{branch}?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{branch}", relBranch,
		"{version}", "1.0")

	urlString := r.Replace(getBranchURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	relBranches := refs{}

	json.NewDecoder(resp.Body).Decode(&relBranches)

	return relBranches
}

func getMasterBranch(client *http.Client) ref {
	getBranchURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/refs/heads/{branch}?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{branch}", secret.MasterBranch,
		"{version}", "1.0")

	urlString := r.Replace(getBranchURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	masterBranches := refs{}

	json.NewDecoder(resp.Body).Decode(&masterBranches)

	fmt.Printf("master branches: %v\n", masterBranches.Count)

	if masterBranches.Count == 0 {
		panic(fmt.Sprintf("No %v branch found", secret.MasterBranch))
	}

	masterBranch := masterBranches.Value[0]
	for i := range masterBranches.Value {
		if masterBranches.Value[i].Name == secret.MasterBranch {
			masterBranch = masterBranches.Value[i]
			break
		}
	}
	return masterBranch
}

func createBranch(client *http.Client, relBranch string, commitID string) {
	newBranch := branch{
		Name:        fmt.Sprintf("%s/%s", "refs/heads", relBranch),
		OldObjectID: "0000000000000000000000000000000000000000",
		NewObjectID: commitID,
	}

	postBranchURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/refs?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "1.0")

	urlString := r.Replace(postBranchURLTemplate)
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode([]branch{newBranch})

	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.Status)
}

func getCommits(client *http.Client, relBranch string, startTime time.Time, endTime time.Time) commits {
	toText, _ := endTime.MarshalText()
	fromText, _ := startTime.MarshalText()
	fmt.Printf("Finding commits from %s to %s...\n", string(fromText), string(toText))

	getCommitsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/commits?api-version={version}&branch={branch}&itemPath={versionPath}&fromDate={fromDateTime}&toDate={toDateTime}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{branch}", relBranch,
		"{versionPath}", secret.VersionPath,
		"{fromDateTime}", string(fromText),
		"{toDateTime}", string(toText),
		"{version}", "1.0")

	urlString := r.Replace(getCommitsURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	commits := commits{}

	json.NewDecoder(resp.Body).Decode(&commits)
	return commits
}

func getBranchVersionXML(client *http.Client, relBranch string) root {
	getItemURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/items?api-version={version}&versionType={versionType}&version={versionValue}&scopePath={versionPath}&lastProcessedChange=true"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{versionType}", "branch",
		"{versionValue}", relBranch,
		"{versionPath}", secret.VersionPath,
		"{version}", "1.0")

	urlString := r.Replace(getItemURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	bodyText, _ := ioutil.ReadAll(resp.Body)

	versionXML := root{}

	err = xml.Unmarshal(bodyText, &versionXML)
	if err != nil {
		log.Fatal(err)
	}
	return versionXML
}

func getCommitVersionXML(client *http.Client, commitID string) root {
	getItemURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/items?api-version={version}&versionType={versionType}&version={versionValue}&scopePath={versionPath}&lastProcessedChange=true"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{versionType}", "commit",
		"{versionValue}", commitID,
		"{versionPath}", secret.VersionPath,
		"{version}", "1.0")

	urlString := r.Replace(getItemURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	bodyText, _ := ioutil.ReadAll(resp.Body)

	versionXML := root{}
	err = xml.Unmarshal(bodyText, &versionXML)
	if err != nil {
		log.Fatal(err)
	}
	return versionXML
}

func resetBuildVersion(client *http.Client, versionXML root, build string, relBranch string, commitID string) {
	versions := strings.Split(versionXML.Versions[0].Value, ".")
	buildNum, err := strconv.Atoi(build)
	if err != nil {
		log.Fatal(err)
	}
	versions[len(versions)-2] = strconv.Itoa(buildNum + 1)
	versions[len(versions)-1] = "0"
	versionXML.Versions[0].Value = strings.Join(versions, ".")
	fmt.Printf("Reset version to: %s\n", versionXML.Versions[0].Value)
	content, err := xml.MarshalIndent(versionXML, "", "  ")

	postPushURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/git/repositories/{repository}/pushes?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{repository}", secret.Repo,
		"{version}", "2.0-preview")

	urlString := r.Replace(postPushURLTemplate)

	versionResetPush := push{
		RefUpdates: []refUpdate{
			{
				Name:        fmt.Sprintf("%s/%s", "refs/heads", relBranch),
				OldObjectID: commitID,
			},
		},
		Commits: []commit{
			{
				Comment: "Reset version for release",
				Changes: []change{
					{
						ChangeType: "edit",
						Item: item{
							Path: secret.VersionPath,
						},
						NewContent: newContent{
							ContentType: "rawtext",
							Content:     string(content),
						},
					},
				},
			},
		},
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(versionResetPush)

	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.Status)
}

func getBuildDefinitions(client *http.Client, relBranch string) definitions {
	getDefinitionsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/definitions?api-version={version}&path={path}&name={definitionName}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "3.0-preview.2",
		"{path}", fmt.Sprintf("%s\\%s", secret.DefinitionPathPrefix, relBranch),
		"{definitionName}", secret.DefinitionName)

	urlString := r.Replace(getDefinitionsURLTemplate)

	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(secret.Username, secret.Password)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	defs := definitions{}
	fmt.Println(resp.Status)
	json.NewDecoder(resp.Body).Decode(&defs)

	return defs
}

func onboardBuildDefinition(client *http.Client, relBranch string) {
	postBuildURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/builds?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "2.0")

	urlString := r.Replace(postBuildURLTemplate)

	onboardBuild := buildReq{
		Definition: definition{
			ID: secret.OnboardBuildDefinitionID,
		},
		SourceBranch: fmt.Sprintf("%s/%s", "refs/heads", "master"),
		Parameters:   fmt.Sprintf("{\"GitRepositoryName\":\"Compute-CloudShell\",\"GitBranchName\":\"%s\"}", relBranch),
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(onboardBuild)

	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}
	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println("Onboarding...")
	fmt.Println(resp.Status)
}

func getBuilds(client *http.Client, defID int) builds {
	getBuildsURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/builds?api-version={version}&definitions={defID}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "2.0",
		"{defID}", strconv.Itoa(defID))

	urlString := r.Replace(getBuildsURLTemplate)
	req, err := http.NewRequest("GET", urlString, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	builds := builds{}
	fmt.Println(resp.Status)
	json.NewDecoder(resp.Body).Decode(&builds)

	return builds
}

func postBuild(client *http.Client, relBranch string, buildDefID int) {
	postBuildURLTemplate := "https://{instance}/DefaultCollection/{project}/_apis/build/builds?api-version={version}"
	r := strings.NewReplacer(
		"{instance}", secret.Instance,
		"{project}", secret.Project,
		"{version}", "2.0")

	urlString := r.Replace(postBuildURLTemplate)

	relBuild := buildReq{
		Definition: definition{
			ID: buildDefID,
		},
		SourceBranch: fmt.Sprintf("%s/%s", "refs/heads", relBranch),
	}
	body := new(bytes.Buffer)
	json.NewEncoder(body).Encode(relBuild)
	req, err := http.NewRequest("POST", urlString, body)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(secret.Username, secret.Password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println("Building...")
	fmt.Println(resp.Status)
}

func main() {
	// read secrets
	file, _ := os.Open("secrets.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	err := decoder.Decode(&secret)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(secret.Username)

	client := &http.Client{}

	// check branch
	commitID := "0000000000000000000000000000000000000000"

	n := time.Now()
	releaseDate := n.AddDate(0, 0, -1*((int(n.Weekday())+6)%7))
	y, m, d := releaseDate.Date()
	relBranch := fmt.Sprintf("%s%v%02v%02v", secret.ReleaseBranchPrefix, y, int(m), d)
	fmt.Println(relBranch)

	relBranches := getRelBranches(client, relBranch)
	fmt.Printf("release branches: %v\n", relBranches.Count)
	if relBranches.Count > 0 {
		fmt.Println("release branch exists.")
		commitID = relBranches.Value[0].ObjectID
	} else {
		// fork
		masterBranch := getMasterBranch(client)

		createBranch(client, relBranch, masterBranch.ObjectID)
		commitID = masterBranch.ObjectID
	}

	// check version
	versionXML := getBranchVersionXML(client, relBranch)

	if len(versionXML.Versions) != 1 {
		fmt.Printf("Error version xml: %+v\n", versionXML)
		return
	}

	if versions := strings.Split(versionXML.Versions[0].Value, "."); versions[len(versions)-1] != "0" {
		fmt.Println(versionXML.Versions[0].Value)
		build := versions[len(versions)-2]
		// check commits
		n = time.Now()
		daysLookBack := 1 + (int(n.Weekday())+6)%7

		commits := getCommits(client, relBranch, n.AddDate(0, 0, -1*daysLookBack), n)
		fmt.Println(commits.Count)

		for _, commit := range commits.Value {
			versionXML := getCommitVersionXML(client, commit.CommitID)

			if len(versionXML.Versions) != 1 {
				fmt.Printf("Found more than one version, commit %s: %+v\n", commit.CommitID, versionXML)
			}

			if versions := strings.Split(versionXML.Versions[0].Value, "."); versions[len(versions)-2] != build {
				fmt.Printf("Found: Commit %s: %+v\n", commit.CommitID, versionXML.Versions[0].Value)

				return
			}
		}
		fmt.Printf("No version reset found in %v days.\n", daysLookBack)

		// reset version
		resetBuildVersion(client, versionXML, build, relBranch, commitID)
	}

	// check master branch version

	// submit PR

	// check PR

	// merge PR

	// check build definition
	defs := getBuildDefinitions(client, relBranch)

	fmt.Printf("%v\n", defs.Count)

	if defs.Count < 1 {
		// create build definition
		onboardBuildDefinition(client, relBranch)

		i := 0
		for ; i < 10; i++ {
			time.Sleep(30 * time.Second)
			defs = getBuildDefinitions(client, relBranch)
			fmt.Printf("%v\n", defs.Count)
			if defs.Count >= 1 {
				break
			}
		}

		if i >= 10 {
			fmt.Printf("No build definitions after %v seconds...\n", i*30)
			return
		}
	}

	buildDefID := defs.Value[0].ID
	for _, def := range defs.Value {
		if def.Name == secret.DefinitionName {
			buildDefID = def.ID
			break
		}
	}

	fmt.Printf("%v\n", buildDefID)

	// check build
	builds := getBuilds(client, buildDefID)
	fmt.Println(builds.Count)
	if builds.Count < 1 {
		// create build
		postBuild(client, relBranch, buildDefID)
	}
}
