package flow

// Examples of each command. A test checks that rayo accepts them all.
var examples = map[string]string{
	"assertNotVisible":      "- assertNotVisible: Loading.*\n- assertNotVisible: { id: spinner, timeout: 10s }",
	"assertTrue":            "- evalScript: ${output.total = 3}\n- assertTrue: ${output.total == 3}",
	"assertVisible":         "- assertVisible: \"Welcome.*\"\n- assertVisible: { id: price, text: \"Price (€)\" }",
	"back":                  "- back",
	"camera":                "- camera: qr.png      # what the emulator's camera sees\n- camera: none        # the room as it was",
	"clearState":            "- clearState\n- clearState: com.example.other",
	"copyTextFrom":          "- copyTextFrom: { id: order_number }\n- tapOn: { id: search }\n- pasteText",
	"doubleTapOn":           "- doubleTapOn: { id: photo }",
	"eraseText":             "- tapOn: { id: email }\n- eraseText           # 50 characters\n- eraseText: 10",
	"evalScript":            "- evalScript: ${output.count = 1}",
	"extendedWaitUntil":     "- extendedWaitUntil: { visible: { id: home }, timeout: 20s }\n- extendedWaitUntil: { notVisible: Loading.* }",
	"hideKeyboard":          "- hideKeyboard",
	"http":                  "- http: https://api.example.com/reset\n- http: { method: PUT, url: \"https://api.example.com/state\", json: { state: ready }, expect: 204 }",
	"inputRandomEmail":      "- tapOn: { id: email }\n- inputRandomEmail",
	"inputRandomNumber":     "- inputRandomNumber\n- inputRandomNumber: { length: 4 }",
	"inputRandomPersonName": "- inputRandomPersonName",
	"inputRandomText":       "- inputRandomText: { length: 20 }",
	"inputText":             "- tapOn: { id: email }\n- inputText: me@example.com\n- inputText: ${EMAIL}",
	"killApp":               "- killApp",
	"launchApp":             "- launchApp\n- launchApp: { clearState: true, permissions: { all: allow }, arguments: { mock: true } }\n- launchApp: { stopApp: false }     # back as it was",
	"longPressOn":           "- longPressOn: { id: message }",
	"mock":                  "- mock: overrides.json\n- mock: { request: { method: POST, urlPathPattern: \".*/payments\" }, response: { status: 503 } }",
	"mockState":             "- mockState: { order: shipped }",
	"network":               "- network: offline\n- network: { delay: 2s }\n- network: { fail: 503, path: \".*/payments\" }\n- network: online",
	"openLink":              "- openLink: https://example.com/item/7",
	"pasteText":             "- copyTextFrom: { id: code }\n- tapOn: { id: code_input }\n- pasteText",
	"pressKey":              "- pressKey: Enter\n- pressKey: Home",
	"repeat":                "- repeat:\n    times: 3\n    commands:\n      - tapOn: { id: next }\n- repeat:\n    while: { visible: { id: next } }\n    times: 10\n    commands:\n      - tapOn: { id: next }",
	"retry":                 "- retry:\n    retries: 2\n    commands:\n      - tapOn: { id: refresh }\n      - assertVisible: Updated",
	"runFlow":               "- runFlow: login.yaml\n- runFlow: { file: login.yaml, env: { EMAIL: me@example.com } }\n- runFlow:\n    when: { visible: Accept cookies }\n    commands:\n      - tapOn: Accept",
	"runScript":             "- runScript: create_user.js\n- runScript: { file: create_user.js, env: { NAME: Ana } }",
	"scroll":                "- scroll",
	"scrollUntilVisible":    "- scrollUntilVisible: { element: { text: Terms }, timeout: 30s }",
	"setAirplaneMode":       "- setAirplaneMode: enabled\n- setAirplaneMode: disabled",
	"setLocation":           "- setLocation: { latitude: 41.3785, longitude: 2.1765 }",
	"setPermissions":        "- setPermissions: { permissions: { all: deny, camera: allow } }",
	"shell":                 "- shell: am broadcast -a com.example.REFRESH -p com.example.app",
	"startRecording":        "- startRecording          # with the flow's name\n- tapOn: { id: login }\n- stopRecording\n- startRecording: checkout",
	"stopApp":               "- stopApp\n- stopApp: com.example.other",
	"stopRecording":         "- startRecording: login\n- tapOn: { id: login }\n- stopRecording",
	"swipe":                 "- swipe: { direction: LEFT }\n- swipe: { direction: UP, from: { id: sheet } }\n- swipe: { start: \"50%, 80%\", end: \"50%, 20%\", duration: 400 }",
	"takeScreenshot":        "- takeScreenshot: home",
	"tapOn":                 "- tapOn: Log in\n- tapOn: { id: save }\n- tapOn: { text: Delete, below: { id: row, text: Invoice 7 } }\n- tapOn: { point: \"35%, 30%\" }",
	"travel":                "- travel: { points: [\"41.3785,2.1765\", \"41.3805,2.1748\"], speed: 6 }",
	"type":                  "- type: { id: email, value: me@example.com }",
	"waitForAnimationToEnd": "- waitForAnimationToEnd\n- waitForAnimationToEnd: { timeout: 3s }",
	"waitForRequest":        "- waitForRequest: \".*/payments\"\n- waitForRequest: { method: POST, path: \".*/payments\", body: \"\\\"amount\\\":5\" }",
}

// KeyDoc is a key, for who reads about it.
type KeyDoc struct {
	Name   string   `json:"name"`
	Takes  string   `json:"takes"`
	About  string   `json:"about"`
	Values []string `json:"values"` // if only some
	Short  bool     `json:"short"`  // what a bare value stands for
}

// CommandDoc is a command, for who reads about it.
type CommandDoc struct {
	Name    string   `json:"name"`
	About   string   `json:"about"`
	Example string   `json:"example"`
	Tested  bool     `json:"tested"` // on a device
	Alone   bool     `json:"alone"`
	Element bool     `json:"element"` // takes a selector
	Keys    []KeyDoc `json:"keys"`
}

// Doc is all that a flow may hold.
type Doc struct {
	Commands []CommandDoc `json:"commands"`
	Selector []KeyDoc     `json:"selector"`
	When     []KeyDoc     `json:"when"`
	Options  []KeyDoc     `json:"options"` // of every command
}

// Describe is for programs, and the documentation's page.
func Describe() Doc {
	d := Doc{Selector: keyDocs(selectorKeys, nil), When: keyDocs(whenKeys, nil), Options: keyDocs(commonKeys, nil)}
	for _, name := range Commands() {
		c := commands[name]
		d.Commands = append(d.Commands, CommandDoc{Name: name, About: c.about, Example: examples[name], Tested: !c.untested,
			Alone: c.alone, Element: c.element, Keys: keyDocs(c.keys, c.plain)})
	}
	return d
}

func keyDocs(keys []anyKey, plain anyKey) []KeyDoc {
	list := []KeyDoc{}
	for _, k := range keys {
		list = append(list, KeyDoc{k.title(), k.takes(), k.describe(), k.values(), k == plain})
	}
	return list
}
