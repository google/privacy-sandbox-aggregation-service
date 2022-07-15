import React from 'react';
import { formatTime } from '../jobs-functions.js';
import VALUES from '../values.js'


// Component for an individual subjob.
const Log = (props) => {

    // cleans code later on
    let log = props.log;
    let job = props.job;

    return (
        <div className="log">
            <div id={job.id + "-" + log.level + "-header"} data-status={log.status} className="log-header" data-id={job.id + "-" + log.level}>
                <span>{log.level}</span>
                <i className={"material-icons " + log.status}>{VALUES.jobStatus[log.status]}</i>
                <i className="material-icons">keyboard_arrow_down</i>
            </div>
            <div id={job.id + "-" + log.level + "-info"} className="log-info">
                <div className="result-container">
                    <p><b>Result</b></p>
                    <p>{log.result=="" ? "N/A" : log.result}</p>
                </div>
                <div className="message-container">
                    <p><b>Message</b></p>
                    <p>{log.message=="" ? "N/A" : log.message}</p>
                </div>
                <div className="created-container">
                    <p><b>Created</b></p>
                    <p>{formatTime(log.created)}</p>
                </div>
                <div className="updated-container">
                    <p><b>Last Updated</b></p>
                    <p>{formatTime(log.updated, true)}</p>
                </div>
            </div>
        </div>
    )
}

export default Log;