import React from 'react';
import VALUES from '../values.js'

const Log = (props) => {
    let log = props.log;  // cleans code later on
    let job = props.job;  // cleans code later on

    return (
        <div className="log">
            <div id={job.id + "-" + log.level + "-header"} data-status={log.status} className="log-header" data-id={job.id + "-" + log.level}>
                <span>{log.level}</span>
                <i className={"material-icons " + log.status}>{VALUES.jobStatus[log.status]}</i>
                <i className="material-icons">keyboard_arrow_down</i>
            </div>
            <div id={job.id + "-" + log.level + "-info"} className="log-info">
                <p><b>Result</b></p>
                <p>{log.result=="" ? "N/A" : log.result}</p>
                <p><b>Message</b></p>
                <p>{log.message=="" ? "N/A" : log.message}</p>
            </div>
        </div>
    )
}

export default Log;