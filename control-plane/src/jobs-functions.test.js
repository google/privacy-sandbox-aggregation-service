// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { addLevel, formatTime, timeSince } from './jobs-functions'
import { Timestamp } from "firebase/firestore";

describe('addLevel Tests', () => {
    test('Testing addLevel (Test 1)', () => {
        // I remove whitespace to ensure that all everything matches
        expect(addLevel(1,1).replace(/\s/g, '')).toBe(`<div id="aggregator-1-level-1" class="level">
        <div id="aggregator-1-level-1-header" class="level-header active-level" data-id="aggregator-1-level-1">
            <span>Level 1</span>
            <i class="material-icons remove-aggregator" data-id="aggregator-1-level-1">close</i><i class="material-icons">keyboard_arrow_down</i>
        </div>
        <div id="aggregator-1-level-1-info" class="mdl-grid level-info">
            <div class="mdl-cell mdl-cell--12-col aggregator-1-level-1">
                <div class="mdl-textfield mdl-js-textfield">
                    <textarea class="mdl-textfield__input" type="text" rows="3"
                        id="aggregator-1-level-1-message-field"></textarea>
                    <label class="mdl-textfield__label" for="aggregator-1-level-1-message-field">Message</label>
                </div>
                <br>
                <div class="mdl-textfield mdl-js-textfield">
                    <textarea class="mdl-textfield__input" type="text" rows="3"
                        id="aggregator-1-level-1-result-field"></textarea>
                    <label class="mdl-textfield__label" for="aggregator-1-level-1-result-field">Result</label>
                </div>
                <br>
                <h6>Current Status</h6>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-1-level-1-option-1">
                    <input type="radio" id="aggregator-1-level-1-option-1" class="mdl-radio__button" name="aggregator-1-level-1-options"
                        value="Scheduled" checked>
                    <span class="mdl-radio__label">Scheduled</span>
                </label>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-1-level-1-option-2">
                    <input type="radio" id="aggregator-1-level-1-option-2" class="mdl-radio__button" name="aggregator-1-level-1-options"
                        value="Running">
                    <span class="mdl-radio__label">Running</span>
                </label>
                <br>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-1-level-1-option-3">
                    <input type="radio" id="aggregator-1-level-1-option-3" class="mdl-radio__button" name="aggregator-1-level-1-options"
                        value="Finished">
                    <span class="mdl-radio__label">Finished</span>
                </label>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-1-level-1-option-4">
                    <input type="radio" id="aggregator-1-level-1-option-4" class="mdl-radio__button" name="aggregator-1-level-1-options"
                        value="Failed">
                    <span class="mdl-radio__label">Failed</span>
                </label>
            </div>
        </div>
    </div>`.replace(/\s/g, ''))
    })
    test('Testing addLevel (Test 2)', () => {
        // I remove whitespace to ensure that all everything matches
        expect(addLevel(2,1).replace(/\s/g, '')).toBe(`<div id="aggregator-2-level-1" class="level">
        <div id="aggregator-2-level-1-header" class="level-header active-level" data-id="aggregator-2-level-1">
            <span>Level 1</span>
            <i class="material-icons remove-aggregator" data-id="aggregator-2-level-1">close</i><i class="material-icons">keyboard_arrow_down</i>
        </div>
        <div id="aggregator-2-level-1-info" class="mdl-grid level-info">
            <div class="mdl-cell mdl-cell--12-col aggregator-2-level-1">
                <div class="mdl-textfield mdl-js-textfield">
                    <textarea class="mdl-textfield__input" type="text" rows="3"
                        id="aggregator-2-level-1-message-field"></textarea>
                    <label class="mdl-textfield__label" for="aggregator-2-level-1-message-field">Message</label>
                </div>
                <br>
                <div class="mdl-textfield mdl-js-textfield">
                    <textarea class="mdl-textfield__input" type="text" rows="3"
                        id="aggregator-2-level-1-result-field"></textarea>
                    <label class="mdl-textfield__label" for="aggregator-2-level-1-result-field">Result</label>
                </div>
                <br>
                <h6>Current Status</h6>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-1-option-1">
                    <input type="radio" id="aggregator-2-level-1-option-1" class="mdl-radio__button" name="aggregator-2-level-1-options"
                        value="Scheduled" checked>
                    <span class="mdl-radio__label">Scheduled</span>
                </label>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-1-option-2">
                    <input type="radio" id="aggregator-2-level-1-option-2" class="mdl-radio__button" name="aggregator-2-level-1-options"
                        value="Running">
                    <span class="mdl-radio__label">Running</span>
                </label>
                <br>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-1-option-3">
                    <input type="radio" id="aggregator-2-level-1-option-3" class="mdl-radio__button" name="aggregator-2-level-1-options"
                        value="Finished">
                    <span class="mdl-radio__label">Finished</span>
                </label>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-1-option-4">
                    <input type="radio" id="aggregator-2-level-1-option-4" class="mdl-radio__button" name="aggregator-2-level-1-options"
                        value="Failed">
                    <span class="mdl-radio__label">Failed</span>
                </label>
            </div>
        </div>
    </div>`.replace(/\s/g, ''))
    })
    test('Testing addLevel (Test 3)', () => {
        // I remove whitespace to ensure that all everything matches
        expect(addLevel(2,50).replace(/\s/g, '')).toBe(`<div id="aggregator-2-level-50" class="level">
        <div id="aggregator-2-level-50-header" class="level-header active-level" data-id="aggregator-2-level-50">
            <span>Level 50</span>
            <i class="material-icons remove-aggregator" data-id="aggregator-2-level-50">close</i><i class="material-icons">keyboard_arrow_down</i>
        </div>
        <div id="aggregator-2-level-50-info" class="mdl-grid level-info">
            <div class="mdl-cell mdl-cell--12-col aggregator-2-level-50">
                <div class="mdl-textfield mdl-js-textfield">
                    <textarea class="mdl-textfield__input" type="text" rows="3"
                        id="aggregator-2-level-50-message-field"></textarea>
                    <label class="mdl-textfield__label" for="aggregator-2-level-50-message-field">Message</label>
                </div>
                <br>
                <div class="mdl-textfield mdl-js-textfield">
                    <textarea class="mdl-textfield__input" type="text" rows="3"
                        id="aggregator-2-level-50-result-field"></textarea>
                    <label class="mdl-textfield__label" for="aggregator-2-level-50-result-field">Result</label>
                </div>
                <br>
                <h6>Current Status</h6>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-50-option-1">
                    <input type="radio" id="aggregator-2-level-50-option-1" class="mdl-radio__button" name="aggregator-2-level-50-options"
                        value="Scheduled" checked>
                    <span class="mdl-radio__label">Scheduled</span>
                </label>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-50-option-2">
                    <input type="radio" id="aggregator-2-level-50-option-2" class="mdl-radio__button" name="aggregator-2-level-50-options"
                        value="Running">
                    <span class="mdl-radio__label">Running</span>
                </label>
                <br>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-50-option-3">
                    <input type="radio" id="aggregator-2-level-50-option-3" class="mdl-radio__button" name="aggregator-2-level-50-options"
                        value="Finished">
                    <span class="mdl-radio__label">Finished</span>
                </label>
                <label class="mdl-radio mdl-js-radio mdl-js-ripple-effect" for="aggregator-2-level-50-option-4">
                    <input type="radio" id="aggregator-2-level-50-option-4" class="mdl-radio__button" name="aggregator-2-level-50-options"
                        value="Failed">
                    <span class="mdl-radio__label">Failed</span>
                </label>
            </div>
        </div>
    </div>`.replace(/\s/g, ''))
    })
})

describe('timeSince Tests', () => {
    test('Testing timeSince (Test 1)', () => {
        var d = new Date();
        d.setDate(d.getDate()-2);
        expect(timeSince(d)).toBe("2 days")
    })
    test('Testing timeSince (Test 2)', () => {
        var d = new Date();
        d.setDate(d.getDate()- 5);
        expect(timeSince(d)).toBe("5 days")
    })
    test('Testing timeSince (Test 3)', () => {
        var d = new Date();
        d.setHours(d.getHours()-2);
        expect(timeSince(d)).toBe("2 hours")
    })
})

describe('formatTime Tests', () => {
    test('Testing formatTime (Test 1)', () => {
        expect(formatTime(Timestamp.fromDate(new Date('1995-12-17T03:24:00')), false)).toBe("12/17/1995");
    })
    test('Testing formatTime (Test 2)', () => {
        expect(formatTime(new Timestamp(1655742137, 0), false)).toBe("6/20/2022")
    })
    test('Testing formatTime (Test 3)', () => {
        var d = new Date();
        d.setHours(d.getHours()-2);
        expect(formatTime(Timestamp.fromDate(d), true)).toBe("2 hours ago");
    })
})