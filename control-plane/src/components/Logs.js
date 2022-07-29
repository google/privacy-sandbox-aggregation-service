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

import React from 'react';
import Log from './Log';

// Component that controls all subjobs in a job
const Logs = (props) => {
    var rows = [];
    let job = props.job;
    for (let i = 0; i < job.logs.length; i++) {
        let log = job.logs[i];
        rows.push(<Log job={job} log={log} key={i} />);
    }
    return (
        <tr id={job.id + "-info"} className="logs">
            <td colSpan={5}>
                <div className="levels-logs">
                    <h5>Sub-job Status</h5>
                    {rows}
                </div>
            </td>
        </tr>
    )
}

export default Logs;