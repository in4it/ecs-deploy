
import { tap } from 'rxjs/operators';
import { Injectable } from '@angular/core';
import { HttpInterceptor, HttpHandler, HttpRequest, HttpEvent, HttpResponse, HttpErrorResponse } from '@angular/common/http';

import { Observable } from 'rxjs';


import { AlertService } from '../services/index';

import { environment } from '../../environments/environment'

@Injectable()
export class AppHttpInterceptor implements HttpInterceptor {

  constructor( private alertService: AlertService) { }

	intercept(request: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
		return next.handle(request).pipe(tap((event: HttpEvent<any>) => {
		  if (event instanceof HttpResponse) {
		    // process successful responses here
		  }
		}, (error: any) => {
		  if (error instanceof HttpErrorResponse) {
		    if (error.status === 401) {
          localStorage.removeItem('token');
          if (environment.samlEnabled) {
            window.location.href = '/ecs-deploy/saml/acs'
            return
          } else {
            // show login
          }
          this.alertService.error("Token expired, log in again");
		    } else if (error.status === 504) {
          this.alertService.error("Couldn't connect to the backend - try again later");
		    }
		  }
		}));
	}
}
