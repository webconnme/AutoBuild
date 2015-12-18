/**
 * The MIT License (MIT)
 *
 * Copyright (c) 2015 David You <david@webconn.me>
 * Copyright (c) 2015 Jane Lee <jane@webconn.me>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

package main

import (
	"fmt"
	"log"
	"os"
	"time"
	"io"
	"github.com/codeskyblue/go-sh"
	"encoding/json"
	"strings"
)

const build_work_dir_perm     = os.FileMode(0777)     // 빌드 디렉토리 퍼미션

type Config struct {
	Frome			string	`json:"from"`
	To				string	`json:"to"`
	Title			string			// 빌드 제목
	RequestTime		string			// 빌드 요청일 : 2015-10-15 10:30:27
	CloneDir		string
	GitRepo			string			// 커널 깃트 레포지트리 URL
	Target			[]Targets
}

type RunConfig struct {
	prompt				string		// 커널 빌드 중 임을 알리는 프롬프트

	buildLogFileName	string		// 빌드 로그 파일 이름
	buildStartTime		string		// 빌드 시  작
	buildEndTime		string		// 빌드 종  료

	buildTopPath		string		// 빌드 가장 상위 디렉토리
	buildLogFilePath	string		// 빌드 로그 파일 패쓰
	buildWorkPath		string		// 빌드 작업 디렉토리
}

type Targets struct {
	Title			string
	SubGitSrc		string
	DockerName		string
	PreCmd			[]string
	BuildCmd		[]string
	PostCmd			[]string
	RstFile			string
}

var defconf		Config;     // 실행 조건
var RunEnv		RunConfig;  // 실행 설정
var logger		*log.Logger // 로거

//---------------------------------------------------------------------------------------------------------------------
//   실행 조건 파일을 읽어와 저장한다.
//---------------------------------------------------------------------------------------------------------------------
func initConfig() {

	b := os.Args[1]

	err := json.Unmarshal([]byte(b),&defconf)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

//---------------------------------------------------------------------------------------------------------------------
//   빌드 상위 패쓰를 구한다.
//---------------------------------------------------------------------------------------------------------------------
func getBuildTopPath() {

	out ,_ :=  sh.Command("pwd").CombinedOutput()
	RunEnv.buildTopPath = strings.Trim(string(out),"\n")+"/"
}

//---------------------------------------------------------------------------------------------------------------------
//   시간 마크  (ex : 2015-11-16T16:26:46 )
//---------------------------------------------------------------------------------------------------------------------
func getTime() string {

	t := time.Now()

	setTime := fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	return setTime;
}

//---------------------------------------------------------------------------------------------------------------------
//   작업 준비를 한다.
//---------------------------------------------------------------------------------------------------------------------
func prepare() bool {

	logger.Printf( "Run Directory [%s]\n",		RunEnv.buildTopPath )
	logger.Printf( "Build Log File  [%s]\n",	RunEnv.buildLogFilePath )

	// 디렉토리 패쓰를 정리한다.
	RunEnv.buildWorkPath	= RunEnv.buildTopPath + "build/"

	logger.Printf( ">>>> Run Directory [%s]\n",		RunEnv.buildTopPath )
	logger.Printf( ">>>> Build Log File  [%s]\n",	RunEnv.buildLogFilePath )

	// 빌드 작업 디렉토리를 새로 생성한다.
	logger.Printf( "Create Directory [%s] \n", RunEnv.buildWorkPath )

	if err := os.MkdirAll( RunEnv.buildWorkPath, build_work_dir_perm ); err != nil {

		logger.Printf( "fail Create Directory [%s] \n", RunEnv.buildWorkPath )
		return false
	}

	// 작업 디렉토리로 이동한다.
	logger.Printf( "Change Build Directory [%s] \n", RunEnv.buildWorkPath )

	if err := os.Chdir( RunEnv.buildWorkPath ); err != nil {
		logger.Printf( "fail Change Directory [%s] \n", RunEnv.buildWorkPath )
		return false
	}

	return true
}

//---------------------------------------------------------------------------------------------------------------------
//   git 에서 소스를 다운로드 한다.
//---------------------------------------------------------------------------------------------------------------------
func git_clone() bool {

	fmt.Printf( "Clone Source [%s]\n", defconf.GitRepo )

	out, err := sh.Command("git", "clone","--recursive",defconf.GitRepo ,".").CombinedOutput()
	if out != nil {
		fmt.Printf( "Clone Log Start\n%s" , out )
		fmt.Printf( "Clone Log End\n" )
	}

	if err != nil {
		fmt.Printf( "Fail Clone Source[%s] \n", defconf.GitRepo )
		return false
	}

	err = sh.Command("cp",("../../autoconfig.sh"),(RunEnv.buildTopPath+"build/buildroot/")).Run()
	if err != nil {
		logger.Println("Fail Result File Copy ")
		return false
	}

	return true
}

//---------------------------------------------------------------------------------------------------------------------
//   빌드실행
//---------------------------------------------------------------------------------------------------------------------
func builder(t *Targets) bool {

	RunEnv.buildWorkPath = RunEnv.buildTopPath+"build/"+ t.SubGitSrc
	logger.Println("change directory : ",RunEnv.buildWorkPath)
	if err := os.Chdir(RunEnv.buildWorkPath); err != nil {
		logger.Printf( "Fail Change Directory [%s] \n", RunEnv.buildWorkPath )
		return false
	}

	logger.Printf("Build Start [%s]\n", RunEnv.buildStartTime);

	if !git_build(t) {
		return false
	}

	return true

}

//---------------------------------------------------------------------------------------------------------------------
//   source를 빌드한다. -> make ARCH=arm  CROSS_COMPILE=arm-generic-linux-gnueabi- uImage O=/work/build/
//---------------------------------------------------------------------------------------------------------------------
func git_build(t *Targets) bool {

	var arg,args []string
	logger.Printf( "Source Directory [%s] \n"	, RunEnv.buildWorkPath )

	DockerPath := RunEnv.buildWorkPath +":/work"
	arg = []string{"run","--rm","--volume",DockerPath,t.DockerName}

	args = append(arg,t.PreCmd...)

	logger.Println("Run Command : ",args)
	out, err := sh.Command("docker",args).CombinedOutput()

	if out != nil {
		logger.Printf( "Compile Log Start\n%s" , out )
		logger.Printf( "Compile Log End\n" )
	}
	if err != nil {
		logger.Printf( "compile error\n%s" , out )
		logger.Printf( "fail Build [%s] \n", t.PreCmd )
		logger.Printf( "Build Result [%s]\n", string(out) )
		return false
	}

	args = append(arg,t.BuildCmd...)
	logger.Println("Run Command : ",args)
	out, err = sh.Command("docker",args).CombinedOutput()

	if out != nil {
		logger.Printf( "Compile Log Start\n%s" , out )
		logger.Printf( "Compile Log End\n" )
	}
	if err != nil {
		logger.Printf( "compile error\n%s" , out )
		logger.Printf( "fail Build [%s] \n", t.BuildCmd )
		logger.Printf( "Build Result [%s]\n", string(out) )
		return false
	}

	args = append(arg,t.PostCmd...)
	logger.Println("Run Command : ",args)
	out, err = sh.Command("docker",args).CombinedOutput()

	if out != nil {
		logger.Printf( "Compile Log Start\n%s" , out )
		logger.Printf( "Compile Log End\n" )
	}
	if err != nil {
		logger.Printf( "compile error\n%s" , out )
		logger.Printf( "fail Build [%s] \n", t.PostCmd )
		logger.Printf( "Build Result [%s]\n", string(out) )
		return false
	}

	logger.Printf( "Build Result [%s]\n", string(out) ) // bug 제거 요청

	err = sh.Command("cp",t.RstFile,RunEnv.buildTopPath).Run()
	if err != nil {
		logger.Println("Fail Result File Copy ",t.RstFile,)
//		return false
	}

	return true
}

//---------------------------------------------------------------------------------------------------------------------
//   이 프로그램은 빌드하고 시험하는 프로그램이다.
//---------------------------------------------------------------------------------------------------------------------
func main() {

	// 빌드 조건을 초기화 한다.
	initConfig()
	getBuildTopPath()

	RunEnv.buildStartTime	 = getTime()  // 빌드 시작 시간을 표기 한다.

	RunEnv.prompt            = fmt.Sprintf( ">>>> %s : ", defconf.Title ) // 빌드 중 임을 알리는 프롬프트
	RunEnv.buildLogFileName  = strings.Trim(defconf.Title," ")+".log"
	RunEnv.buildLogFilePath  = RunEnv.buildTopPath + RunEnv.buildLogFileName

	logFile, err := os.OpenFile(RunEnv.buildLogFilePath, os.O_CREATE | os.O_WRONLY | os.O_APPEND, 0666)
	if err != nil {
		log.Fatalln("Failed to open log file", RunEnv.buildLogFilePath, ":", err)
	}

	multiLog := io.MultiWriter(logFile, os.Stdout)
	logger = log.New(multiLog, RunEnv.prompt, log.Ldate | log.Ltime)

	// 빌드 시작
	logger.Printf( "AutoBuild Start [%s]\n", RunEnv.buildStartTime );

	var success bool = true

	if success { success = prepare() }
	if success { success = git_clone() }

	for _, rt := range defconf.Target {
		if success { success = builder(&rt) }
	}

	// 빌드 종료
	RunEnv.buildEndTime = getTime()  // 빌드 종료 시간을 표기 한다.

	logger.Printf("Build End [%s]\n", RunEnv.buildEndTime);
	logFile.Sync()        			// 최소한 여기 까지 로그 파일을 저장하고 동기화 한다.

	logFile.Close()

	if success == false {
		os.Exit(1)
	}
}


