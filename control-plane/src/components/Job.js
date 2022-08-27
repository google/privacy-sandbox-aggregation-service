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
import VALUES from '../values'
import { formatTime } from '../jobs-functions';

// Job component for the table
const Job = (props) => {
    let job = props.job;

    return (
        <tr id={job.id} className="job" data-status={job.status}>
            <td className="mdl-data-table__cell--non-numeric">{formatTime(job.created)}</td>
            <td className="mdl-data-table__cell--non-numeric"><i className={"material-icons " + job.status}>{VALUES.jobStatusIcons[job.status]}</i></td>
            <td className="mdl-data-table__cell--non-numeric">{job.id}</td>
            <td className="mdl-data-table__cell--non-numeric">{formatTime(job.updated, true)}</td>
            <td>
                <i className="material-icons" data-status={job.status} id={job.id + "-dropdown"}>keyboard_arrow_down</i>
            </td>
        </tr>
    );
}

export default Job;