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
import Job from './Job';
import Logs from './Logs';

// component for both the job and the subjobs beneath
const JobAndLogs = (props) => {
    return (
        <>
            <Job job={props.job} key={props.job.id + "-job" } />
            <Logs job={props.job} key={props.job.id + "-logs"} />
        </>
    )   
}

export default JobAndLogs;