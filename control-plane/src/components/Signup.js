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
import { signInWithPopup, GoogleAuthProvider, GithubAuthProvider, createUserWithEmailAndPassword } from "firebase/auth";
import VALUES from "../values";
import { doc, setDoc, Timestamp } from "firebase/firestore";

function SignUp({ parent }) {

    function updatePage() {
        parent.setState( { login: true } )
    }

    function googleSignUp() {
        const provider = new GoogleAuthProvider();
        signUserUp(provider)
    }

    function githubSignUp() {
        const provider = new GithubAuthProvider();
        signUserUp(provider)
    }

    function signUserUp(provider) {
        if(VALUES.auth != null && VALUES.db != null) {
            signInWithPopup(VALUES.auth, provider)
                .then(async (result) => {
                    // The signed-in user info.
                    const user = result.user;
                    VALUES.user = user;
                    
                    await setDoc(doc(VALUES.db, "user-management", "users", "pending-users", user.uid), {"time": Timestamp.now(), "email": user.email})
                }).catch((error) => {
                    // Handle Errors here.
                    const errorCode = error.code;
                    const errorMessage = error.message;
                    console.error("Failed with error code: "+errorCode+". Message: "+errorMessage)
                });
        }
    }

    function emailPasswordSignUp() {
        var email = $('#email').val();
        var password = $("#password").val();
        createUserWithEmailAndPassword(VALUES.auth, email, password)
            .then(async (userCredential) => {
                // Signed in 
                const user = userCredential.user;
                if(user.email == "admin@chromium.org") {
                    await setDoc(doc(VALUES.db, "user-management", "users", "existing-users", user.uid), {"role": "admin", "time": Timestamp.now(), "email": user.email})
                } else {
                    await setDoc(doc(VALUES.db, "user-management", "users", "pending-users", user.uid), {"time": Timestamp.now(), "email": user.email})
                }
            })
            .catch((error) => {
                const errorCode = error.code;
                const errorMessage = error.message;
                console.error("Failed with error code: "+errorCode+". Message: "+errorMessage)
            });
    }

    return (
        <div>
            <img src="./images/controlplane.png" id="main-icon" />
            <h3>Aggregation Jobs Control Plane</h3>
            <div className="social-logins" id="social-logins">
                <button onClick={ googleSignUp }>
                    <img src="https://developers.google.com/static/identity/images/g-logo.png"/><span>Sign up with Google</span>
                </button>
                <button onClick={ githubSignUp }>
                    <div className='github-logo-crop'>
                        <img src="https://github.githubassets.com/images/modules/logos_page/GitHub-Mark.png" className="github-logo"/>
                    </div>
                    <span>Sign up with Github</span>
                </button>
            </div>
            <br />
            <div className="or-text-container">
                <p className="or-text">OR</p>
            </div>
            <div className="mdl-textfield mdl-js-textfield mdl-textfield--floating-label">
                <input className="mdl-textfield__input" type="text" id="email" />
                <label className="mdl-textfield__label" htmlFor="email">Email</label>
            </div>
            <br />
            <div className="mdl-textfield mdl-js-textfield mdl-textfield--floating-label">
                <input className="mdl-textfield__input" type="password" id="password" />
                <label className="mdl-textfield__label" htmlFor="password">Password</label>
            </div>
            <br />
            <button onClick={ emailPasswordSignUp } className="mdl-button mdl-js-button mdl-button--raised mdl-button--colored login-button" type="button">
                Sign Up
            </button>
            <br />
            <p className="move-to">Already have an account? <a href="#" onClick={ updatePage }>Log in</a></p>
        </div>
    );
}

export default SignUp;