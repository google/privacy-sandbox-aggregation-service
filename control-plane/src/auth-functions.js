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
import { createRoot } from 'react-dom/client';
import Authentication from './components/Auth';
import WaitingScreen from './components/Waiting';
import UserManagement from './components/UserManagement';
import './styles/auth.css';
import VALUES from './values';
import { signOut } from 'firebase/auth';

export function showAuthScreen() {
    const container = document.querySelector('#render-content');
    const tableRoot = createRoot(container);
    tableRoot.render(<Authentication />);
}

export function showWaitingScreen() {
    const container = document.querySelector('#render-content');
    const tableRoot = createRoot(container);
    tableRoot.render(<WaitingScreen />);
}

export function showUserManagementScreen() {
  const container = document.querySelector('#render-content');
  const tableRoot = createRoot(container);
  tableRoot.render(<UserManagement />);
}

export function signOutUser() {
    signOut(VALUES.auth).then(() => {
        // Sign-out successful.
        window.jobsTable = undefined;
        const container = document.querySelector('#render-content');
        const tableRoot = createRoot(container);
        tableRoot.render(<Authentication />);
      }).catch((error) => {
        // An error happened.
        console.error(error)
      });
}
