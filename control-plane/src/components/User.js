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
import { formatTime } from '../jobs-functions';

// User component for the table
const User = (props) => {
    let user = props.user;

    return (
        <tr id={user.id} className="user">
            <td className="mdl-data-table__cell--non-numeric">{formatTime(user.created)}</td>
            <td className="mdl-data-table__cell--non-numeric">{user.role}</td>
            <td className="mdl-data-table__cell--non-numeric">{user.id}</td>
            <td className="mdl-data-table__cell--non-numeric">{user.email}</td>
            <td>
                <i className="material-icons"  id={user.id + "-delete"}>delete</i>
            </td>
        </tr>
    );
}

export default User;