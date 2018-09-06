


import { BehaviorSubject } from 'rxjs';
import {HttpClient, HttpHeaders } from '@angular/common/http';
import { AuthService } from '../services/auth.service';


export class ServiceList {
  constructor(public services: string[]) { }
}

import { Injectable } from '@angular/core';

@Injectable()
export class ServiceListService {

  private sl$: BehaviorSubject<ServiceList>
  private sl: ServiceList = new ServiceList([])

  constructor(private http: HttpClient, private auth: AuthService) { } 

  getServiceList() {
    this.sl$ = new BehaviorSubject<ServiceList>(new ServiceList([]))
    this.getServices().subscribe(data => {
      // Read the result field from the JSON response.
      this.sl.services = data['services'];
      this.sl$.next(this.sl)
      this.sl$.complete()
    });
    return this.sl$
  }

  getServices() {
    return this.http.get('/ecs-deploy/api/v1/service/describe', {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
}
