
import {map} from 'rxjs/operators';
import { Injectable } from '@angular/core';
import {HttpClient, HttpHeaders } from '@angular/common/http';
import { Observable } from 'rxjs';


@Injectable()
export class AuthService {
  constructor(private http: HttpClient) { }
	
	getToken(): string {
		return localStorage.getItem("token")
	}
	setToken(token: string): void {
    localStorage.setItem('token', token);
	}

  login(username: string, password: string) {
    return this.http.post('/ecs-deploy/login', {username: username, password: password }).pipe(
      map((response: Response) => {
        // login successful if there's a jwt token in the response
        let res = response;
        if (res && res["token"]) {
          // store user details and jwt token in local storage to keep user logged in between page refreshes
          localStorage.setItem('token', res["token"]);
        }
      }));
  }

  logout() {
    // remove user from local storage to log user out
    localStorage.removeItem('token');
  }
}
