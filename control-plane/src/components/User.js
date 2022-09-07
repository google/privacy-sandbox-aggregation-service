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
import VALUES from '../values';
import { doc, deleteDoc, setDoc, Timestamp } from "firebase/firestore";

// User component for the table
const User = (props) => {
    let user = props.user;
    let parent = props.parent;

    async function deleteUser() {
        await deleteDoc(doc(VALUES.db, "user-management", "users", "existing-users", user.id));
        parent.setState({users: parent.state.users.filter(function(u) { 
            return u.key !== user.id
        })});
        await setDoc(doc(VALUES.db, "user-management", "users", "pending-users", user.id), {"time": Timestamp.now(), "email": user.email})
    }

    return (
        <tr id={user.id} className="user">
            <td className="mdl-data-table__cell--non-numeric">{formatTime(user.created)}</td>
            <td className="mdl-data-table__cell--non-numeric">{user.role}</td>
            <td className="mdl-data-table__cell--non-numeric">{user.id}</td>
            <td className="mdl-data-table__cell--non-numeric">{user.email}</td>
            <td>
                {user.email == "admin@chromium.org" ? null : <i className="material-icons delete-user" onClick={ deleteUser }  id={user.id + "-delete"}>delete</i>}
            </td>
        </tr>
    );
}

export default User;