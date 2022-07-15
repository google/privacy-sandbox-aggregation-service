import React from 'react';

// For update page which is not currently implemented.
const Level = (props) => {

    // cleans code later on
    let level = props.level;
    let currentLevel = props.currentLevel;
    let aggregator = props.aggregator;

    return (
        <div id={"aggregator-" + aggregator + "-level-" + currentLevel} className="level">
            <div id={"aggregator-"+ aggregator + "-level-" + currentLevel + "-header"} className="level-header active-level" data-id={"aggregator-" + aggregator + "-level-" + currentLevel}>
                <span>Level {currentLevel}</span>
                <i className="material-icons">keyboard_arrow_down</i>
            </div>
            <div id={"aggregator-" + aggregator + "-level-" + currentLevel + "-info"} className="mdl-grid level-info">
                <div className={"mdl-cell mdl-cell--12-col aggregator-" + aggregator + "-level-" + currentLevel}>
                    <div className="mdl-textfield mdl-js-textfield is-dirty">
                        <textarea className="mdl-textfield__input" type="text" rows="3"
                            id={"aggregator-" + aggregator + "-level-" + currentLevel + "-message-field"} defaultValue={level.message}></textarea>
                        <label className="mdl-textfield__label" htmlFor={"aggregator-" + aggregator + "-level-" + currentLevel + "-message-field"}>Message</label>
                    </div>
                    <br />
                    <div className="mdl-textfield mdl-js-textfield is-dirty">
                        <textarea className="mdl-textfield__input" type="text" rows="3"
                            id={"aggregator-" + aggregator + "-level-" + currentLevel + "-result-field"} defaultValue={level.result}></textarea>
                        <label className="mdl-textfield__label" htmlFor={"aggregator-" + aggregator + "-level-" + currentLevel + "-result-field"}>Result</label>
                    </div>
                    <br />
                    <h6>Current Status</h6>
                    <label className="mdl-radio mdl-js-radio mdl-js-ripple-effect" htmlFor={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-1"}>
                        <input type="radio" id={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-1"} className="mdl-radio__button" name={"aggregator-" + aggregator + "-level-" + currentLevel + "-options"}
                            defaultValue="Scheduled" defaultChecked={level.status == "scheduled"} />
                        <span className="mdl-radio__label">Scheduled</span>
                    </label>
                    <label className="mdl-radio mdl-js-radio mdl-js-ripple-effect" htmlFor={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-2"}>
                        <input type="radio" id={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-2"} className="mdl-radio__button" name={"aggregator-" + aggregator + "-level-" + currentLevel + "-options"}
                            defaultValue="Running" defaultChecked={level.status == "running"} />
                        <span className="mdl-radio__label">Running</span>
                    </label>
                    <br />
                    <label className="mdl-radio mdl-js-radio mdl-js-ripple-effect" htmlFor={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-3"}>
                        <input type="radio" id={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-3"} className="mdl-radio__button" name={"aggregator-" + aggregator + "-level-" + currentLevel + "-options"}
                            defaultValue="Finished" defaultChecked={level.status == "finished"} />
                        <span className="mdl-radio__label">Finished</span>
                    </label>
                    <label className="mdl-radio mdl-js-radio mdl-js-ripple-effect" htmlFor={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-4"}>
                        <input type="radio" id={"aggregator-" + aggregator + "-level-" + currentLevel + "-option-4"} className="mdl-radio__button" name={"aggregator-" + aggregator + "-level-" + currentLevel + "-options"}
                            defaultValue="Failed" defaultChecked={level.status == "failed"} />
                        <span className="mdl-radio__label">Failed</span>
                    </label>
                </div>
            </div>
        </div>
    );
}

export default Level;