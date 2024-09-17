// selenium_cli.go
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/tebeka/selenium"
	"github.com/tebeka/selenium/chrome"
	"github.com/tebeka/selenium/firefox"
)

// Step defines a single action in the JSON steps
type Step struct {
	Action          string                 `json:"action"`
	Selector        string                 `json:"selector,omitempty"`
	URL             string                 `json:"url,omitempty"`
	Text            string                 `json:"text,omitempty"`
	Timeout         int                    `json:"timeout,omitempty"`
	Filename        string                 `json:"filename,omitempty"`
	Script          string                 `json:"script,omitempty"`
	Params          map[string]interface{} `json:"params,omitempty"`
	WaitDuration    int                    `json:"wait_duration,omitempty"`
	Keys            []string               `json:"keys,omitempty"`
	Value           string                 `json:"value,omitempty"`
	OtherKeys       []string               `json:"other_keys,omitempty"`
	StoreResultAs   string                 `json:"store_result_as,omitempty"`
	Message         string                 `json:"message,omitempty"`
	ExpectedValue   string                 `json:"expected_value,omitempty"`
	ElementSelector string                 `json:"element_selector,omitempty"`
}

// JSONData represents the entire JSON structure
type JSONData []Step

// Context holds the Selenium WebDriver and other runtime data
type Context struct {
	WebDriver selenium.WebDriver
	Variables map[string]string
}

func main() {
	// Define command-line flags
	browserFlag := flag.String("browser", "firefox", "Browser to use (firefox, chrome, edge)")
	webdriverPathFlag := flag.String("webdriver-path", "", "Path to the WebDriver executable (overrides default PATH lookup)")
	headlessFlag := flag.Bool("headless", false, "Run browser in headless mode")
	windowWidthFlag := flag.Int("window-width", 1280, "Width of the browser window")
	windowHeightFlag := flag.Int("window-height", 800, "Height of the browser window")
	timeoutFlag := flag.Int("default-timeout", 30, "Default timeout in seconds for actions")
	portFlag := flag.Int("port", 13337, "Default port for webdriver service")
	closeBrowserFlag := flag.Bool("close", false, "Close the browser after execution")
	flag.Parse()

	// Validate browser flag
	supportedBrowsers := map[string]bool{
		"firefox": true,
		"chrome":  true,
		"edge":    true,
	}
	browser := strings.ToLower(*browserFlag)
	if !supportedBrowsers[browser] {
		log.Fatalf("Unsupported browser: %s. Supported browsers are: firefox, chrome, edge.", browser)
	}

	// Read JSON from stdin
	jsonData, err := readJSONFromStdin()
	if err != nil {
		log.Fatalf("Failed to read JSON from stdin: %v", err)
	}

	// Initialize Selenium WebDriver
	wd, service, err := initializeWebDriver(browser, *webdriverPathFlag, *headlessFlag, *windowWidthFlag, *windowHeightFlag, *timeoutFlag, *portFlag)
	if err != nil || wd == nil {
		log.Fatalf("Failed to initialize WebDriver: %v", err)
	}
	defer func() {
		if wd == nil {
			return
		}
		if *closeBrowserFlag {
			if err := wd.Quit(); err != nil {
				log.Printf("Error quitting WebDriver: %v", err)
			}
		}
		if service != nil {
			service.Stop()
		}
	}()

	ctx := &Context{
		WebDriver: wd,
		Variables: make(map[string]string),
	}

	// Execute each step
	for idx, step := range jsonData {
		fmt.Printf("Executing step %d: %s\n", idx, step.Action)
		if err := executeStep(ctx, step); err != nil {
			log.Fatalf("Error executing step %d (%s): %v", idx, step.Action, err)
		}
	}

	fmt.Println("All steps executed successfully.")
}

// readJSONFromStdin reads all data from stdin and unmarshals it into JSONData
func readJSONFromStdin() (JSONData, error) {
	reader := bufio.NewReader(os.Stdin)
	var sb strings.Builder
	for {
		input, err := reader.ReadString('\n')
		sb.WriteString(input)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading stdin: %v", err)
		}
	}
	var jsonData JSONData
	if err := json.Unmarshal([]byte(sb.String()), &jsonData); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %v", err)
	}
	return jsonData, nil
}

// initializeWebDriver sets up the Selenium WebDriver based on the provided flags
func initializeWebDriver(browser, webdriverPath string, headless bool, width, height, timeout, port int) (selenium.WebDriver, *selenium.Service, error) {
	var service *selenium.Service
	var err error
	var caps selenium.Capabilities
	selenium.SetDebug(true)
	// Define browser-specific capabilities
	switch browser {
	case "firefox":
		caps = selenium.Capabilities{"browserName": "firefox"}
		firefoxCaps := firefox.Capabilities{
			Args: []string{},
		}
		if headless {
			firefoxCaps.Args = append(firefoxCaps.Args, "-headless")
		}
		caps.AddFirefox(firefoxCaps)
	case "chrome":
		caps = selenium.Capabilities{"browserName": "chrome"}
		chromeCaps := chrome.Capabilities{
			Args: []string{},
		}
		if headless {
			chromeCaps.Args = append(chromeCaps.Args, "--headless")
		}
		caps.AddChrome(chromeCaps)
	default:
		return nil, nil, fmt.Errorf("unsupported browser: %s", browser)
	}

	// Start a WebDriver server instance
	service, err = startWebDriverService(browser, webdriverPath, port)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start WebDriver service: %v", err)
	}

	// Connect to the WebDriver instance running locally.
	wd, err := selenium.NewRemote(selenium.Capabilities{"alwaysMatch": caps}, fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		return nil, nil, First[error](
			service.Stop(),
			fmt.Errorf("failed to resize window: %v", err),
		)
	}
	// Set window size
	if err = wd.ResizeWindow("", width, height); err != nil {

		return nil, nil, First[error](
			wd.Quit(),
			service.Stop(),
			fmt.Errorf("failed to resize window: %v", err),
		)
	}

	// Set implicit wait timeout
	if err = wd.SetImplicitWaitTimeout(time.Duration(timeout) * time.Second); err != nil {

		return nil, nil, First[error](
			wd.Quit(),
			service.Stop(),
			fmt.Errorf("failed to resize window: %v", err),
		)
	}

	return wd, service, nil
}

// startWebDriverService starts the appropriate WebDriver service based on the browser
func startWebDriverService(browser, webdriverPath string, port int) (*selenium.Service, error) {
	var service *selenium.Service
	var err error

	switch browser {
	case "firefox":
		if webdriverPath == "" {
			// Assume geckodriver is in PATH
			webdriverPath = "geckodriver"
		}

		service, err = selenium.NewGeckoDriverService(webdriverPath, port, selenium.Output(os.Stderr))
	case "chrome":
		if webdriverPath == "" {
			// Assume chromedriver is in PATH
			webdriverPath = "chromedriver"
		}
		service, err = selenium.NewChromeDriverService(webdriverPath, port, selenium.Output(os.Stderr))

	default:
		return nil, fmt.Errorf("unsupported browser: %s", browser)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to start WebDriver service for %s: %v", browser, err)
	}

	return service, nil
}

// executeStep performs the action defined in a single step
func executeStep(ctx *Context, step Step) error {
	fmt.Printf("Executing action: %s\n", step.Action)
	switch step.Action {
	case "navigate":
		return navigate(ctx, step)
	case "click":
		return click(ctx, step)
	case "double_click":
		return doubleClick(ctx, step)
	case "right_click":
		return rightClick(ctx, step)
	case "enter_text":
		return enterText(ctx, step)
	case "clear":
		return clearText(ctx, step)
	case "select_option":
		return selectOption(ctx, step)
	case "deselect_option":
		return deselectOption(ctx, step)
	case "get_text":
		return getText(ctx, step)
	case "get_attribute":
		return getAttribute(ctx, step)
	case "wait":
		return waitDuration(step)
	case "screenshot":
		return takeScreenshot(ctx, step)
	case "execute_script":
		return executeScript(ctx, step)
	case "scroll":
		return scroll(ctx, step)
	case "hover":
		return hover(ctx, step)
	case "drag_and_drop":
		return dragAndDrop(ctx, step)
	case "switch_to_frame":
		return switchToFrame(ctx, step)
	case "switch_to_default_content":
		return switchToDefaultContent(ctx)
	case "close_browser":
		return closeBrowser(ctx)
	case "quit_browser":
		return quitBrowser(ctx)
	case "assert_title":
		return assertTitle(ctx, step)
	case "assert_element_present":
		return assertElementPresent(ctx, step)
	case "print":
		return printMessage(ctx, step)
	default:
		return fmt.Errorf("unknown action: %s", step.Action)
	}
}

// Action Handlers

func navigate(ctx *Context, step Step) error {
	if step.URL == "" {
		return errors.New("navigate action requires 'url'")
	}
	return ctx.WebDriver.Get(step.URL)
}

func click(ctx *Context, step Step) error {
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	return elem.Click()
}

func doubleClick(ctx *Context, step Step) error {
	_, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	return ctx.WebDriver.DoubleClick()
}

func rightClick(ctx *Context, step Step) error {
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	// Perform right click via JavaScript
	script := "var evt = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, view: window }); arguments[0].dispatchEvent(evt);"
	_, err = ctx.WebDriver.ExecuteScript(script, []interface{}{elem})
	return err
}

func enterText(ctx *Context, step Step) error {
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	return elem.SendKeys(step.Text)
}

func clearText(ctx *Context, step Step) error {
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	return elem.Clear()
}

func selectOption(ctx *Context, step Step) error {
	if step.Params == nil {
		return errors.New("select_option action requires 'params'")
	}
	value, ok := step.Params["value"]
	if !ok {
		return errors.New("select_option action requires 'params.value'")
	}
	valueStr, ok := value.(string)
	if !ok {
		return errors.New("'value' should be a string")
	}

	// Find the select element
	selectElem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}

	// Find the option with the specified value
	optionSelector := fmt.Sprintf("option[value='%s']", valueStr)
	optionElem, err := selectElem.FindElement(selenium.ByCSSSelector, optionSelector)
	if err != nil {
		return fmt.Errorf("option with value '%s' not found", valueStr)
	}

	// Click the option to select it
	return optionElem.Click()
}

func deselectOption(ctx *Context, step Step) error {
	if step.Params == nil {
		return errors.New("deselect_option action requires 'params'")
	}
	value, ok := step.Params["value"]
	if !ok {
		return errors.New("deselect_option action requires 'params.value'")
	}
	valueStr, ok := value.(string)
	if !ok {
		return errors.New("'value' should be a string")
	}

	// Find the select element
	selectElem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}

	// Find the option with the specified value
	optionSelector := fmt.Sprintf("option[value='%s']", valueStr)
	optionElem, err := selectElem.FindElement(selenium.ByCSSSelector, optionSelector)
	if err != nil {
		return fmt.Errorf("option with value '%s' not found", valueStr)
	}

	// Deselect the option by clicking it (if multi-select)
	// Note: The tebeka/selenium package does not provide a direct Deselect method
	// We'll use JavaScript to deselect the option
	script := "arguments[0].selected = false;"
	_, err = ctx.WebDriver.ExecuteScript(script, []interface{}{optionElem})
	if err != nil {
		return fmt.Errorf("failed to deselect option with value '%s': %v", valueStr, err)
	}

	return nil
}

func getText(ctx *Context, step Step) error {
	if step.StoreResultAs == "" {
		return errors.New("get_text action requires 'store_result_as'")
	}
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	text, err := elem.Text()
	if err != nil {
		return err
	}
	ctx.Variables[step.StoreResultAs] = text
	return nil
}

func getAttribute(ctx *Context, step Step) error {
	if step.StoreResultAs == "" {
		return errors.New("get_attribute action requires 'store_result_as'")
	}
	if step.Params == nil {
		return errors.New("get_attribute action requires 'params'")
	}
	attr, ok := step.Params["attribute"]
	if !ok {
		return errors.New("get_attribute action requires 'params.attribute'")
	}
	attrStr, ok := attr.(string)
	if !ok {
		return errors.New("'attribute' should be a string")
	}
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	value, err := elem.GetAttribute(attrStr)
	if err != nil {
		return err
	}
	ctx.Variables[step.StoreResultAs] = value
	return nil
}

func waitDuration(step Step) error {
	duration := time.Duration(step.WaitDuration) * time.Second
	time.Sleep(duration)
	return nil
}

func takeScreenshot(ctx *Context, step Step) error {
	filename := step.Filename
	if filename == "" {
		filename = fmt.Sprintf("screenshot_%d.png", time.Now().Unix())
	}
	png, err := ctx.WebDriver.Screenshot()
	if err != nil {
		return err
	}
	return os.WriteFile(filename, png, 0644)
}

func executeScript(ctx *Context, step Step) error {
	if step.Script == "" {
		return errors.New("execute_script action requires 'script'")
	}
	args := []interface{}{}
	result, err := ctx.WebDriver.ExecuteScript(step.Script, args)
	if err != nil {
		return err
	}
	if step.StoreResultAs != "" {
		ctx.Variables[step.StoreResultAs] = fmt.Sprintf("%v", result)
	}
	return nil
}

func scroll(ctx *Context, step Step) error {
	if step.Params == nil {
		return errors.New("scroll action requires 'params'")
	}
	direction, ok := step.Params["direction"]
	if !ok {
		return errors.New("scroll action requires 'params.direction'")
	}
	directionStr, ok := direction.(string)
	if !ok {
		return errors.New("'direction' should be a string")
	}

	var script string
	switch strings.ToLower(directionStr) {
	case "up":
		script = "window.scrollBy(0, -100);"
	case "down":
		script = "window.scrollBy(0, 100);"
	case "left":
		script = "window.scrollBy(-100, 0);"
	case "right":
		script = "window.scrollBy(100, 0);"
	default:
		return errors.New("invalid scroll direction")
	}

	_, err := ctx.WebDriver.ExecuteScript(script, nil)
	return err
}

func hover(ctx *Context, step Step) error {
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	return elem.MoveTo(0, 0)
}

func dragAndDrop(ctx *Context, step Step) error {
	if step.Params == nil {
		return errors.New("drag_and_drop action requires 'params'")
	}
	sourceSelector, ok := step.Params["source_selector"]
	if !ok {
		return errors.New("drag_and_drop action requires 'params.source_selector'")
	}
	targetSelector, ok := step.Params["target_selector"]
	if !ok {
		return errors.New("drag_and_drop action requires 'params.target_selector'")
	}
	sourceSel, ok := sourceSelector.(string)
	if !ok {
		return errors.New("'source_selector' should be a string")
	}
	targetSel, ok := targetSelector.(string)
	if !ok {
		return errors.New("'target_selector' should be a string")
	}

	sourceElem, err := findElement(ctx, sourceSel, step.Timeout)
	if err != nil {
		return err
	}
	targetElem, err := findElement(ctx, targetSel, step.Timeout)
	if err != nil {
		return err
	}

	// Perform drag and drop via JavaScript
	script := `
	function simulateDragDrop(sourceNode, destinationNode) {
	    var EVENT_TYPES = {
	        DRAG_END: 'dragend',
	        DRAG_START: 'dragstart',
	        DROP: 'drop'
	    }

	    function createCustomEvent(type) {
	        var event = new CustomEvent("CustomEvent")
	        event.initCustomEvent(type, true, true, null)
	        event.dataTransfer = {
	            data: {},
	            setData: function(type, val) {
	                this.data[type] = val
	            },
	            getData: function(type) {
	                return this.data[type]
	            }
	        }
	        return event
	    }

	    function dispatchEvent(node, type, event) {
	        if (node.dispatchEvent) {
	            return node.dispatchEvent(event)
	        }
	        if (node.fireEvent) {
	            return node.fireEvent("on" + type, event)
	        }
	    }

	    var dragStartEvent = createCustomEvent(EVENT_TYPES.DRAG_START)
	    dispatchEvent(sourceNode, EVENT_TYPES.DRAG_START, dragStartEvent)

	    var dropEvent = createCustomEvent(EVENT_TYPES.DROP)
	    dropEvent.dataTransfer = dragStartEvent.dataTransfer
	    dispatchEvent(destinationNode, EVENT_TYPES.DROP, dropEvent)

	    var dragEndEvent = createCustomEvent(EVENT_TYPES.DRAG_END)
	    dragEndEvent.dataTransfer = dragStartEvent.dataTransfer
	    dispatchEvent(sourceNode, EVENT_TYPES.DRAG_END, dragEndEvent)
	}
	simulateDragDrop(arguments[0], arguments[1])
	`
	_, err = ctx.WebDriver.ExecuteScript(script, []interface{}{sourceElem, targetElem})
	return err
}

func switchToFrame(ctx *Context, step Step) error {
	if step.Selector == "" {
		return errors.New("switch_to_frame action requires 'selector' for the iframe")
	}
	elem, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return err
	}
	return ctx.WebDriver.SwitchFrame(elem)
}

func switchToDefaultContent(ctx *Context) error {
	return ctx.WebDriver.SwitchFrame("")
}

func closeBrowser(ctx *Context) error {
	return ctx.WebDriver.Close()
}

func quitBrowser(ctx *Context) error {
	return ctx.WebDriver.Quit()
}

func assertTitle(ctx *Context, step Step) error {
	expected := step.ExpectedValue
	if expected == "" {
		return errors.New("assert_title action requires 'expected_value'")
	}
	title, err := ctx.WebDriver.Title()
	if err != nil {
		return err
	}
	if title != expected {
		return fmt.Errorf("title assertion failed: expected '%s', got '%s'", expected, title)
	}
	return nil
}

func assertElementPresent(ctx *Context, step Step) error {
	if step.Selector == "" {
		return errors.New("assert_element_present action requires 'selector'")
	}
	_, err := findElement(ctx, step.Selector, step.Timeout)
	if err != nil {
		return fmt.Errorf("element '%s' not found", step.Selector)
	}
	return nil
}

func printMessage(ctx *Context, step Step) error {
	message := step.Message
	// Replace placeholders with variable values
	for key, value := range ctx.Variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		message = strings.ReplaceAll(message, placeholder, value)
	}
	fmt.Println(message)
	return nil
}

// Helper Functions

// findElement locates an element using the provided selector and waits up to timeout seconds
func findElement(ctx *Context, selector string, timeout int) (selenium.WebElement, error) {
	if selector == "" {
		return nil, errors.New("selector is required to find an element")
	}
	waitTimeout := time.Duration(timeout) * time.Second
	endTime := time.Now().Add(waitTimeout)

	for {
		elem, err := ctx.WebDriver.FindElement(selenium.ByCSSSelector, selector)
		if err == nil {
			return elem, nil
		}
		if time.Now().After(endTime) {
			return nil, fmt.Errorf("element with selector '%s' not found after %d seconds", selector, timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func First[T any](t ...T) T {
	var defaultVal T
	for _, v := range t {
		return v
	}
	return defaultVal
}
