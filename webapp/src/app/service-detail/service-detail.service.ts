import 'rxjs/add/observable/of';
import 'rxjs/add/operator/map';
import 'rxjs/add/operator/catch';
import { BehaviorSubject } from 'rxjs/BehaviorSubject';
import {HttpClient, HttpHeaders } from '@angular/common/http';
import { AuthService } from '../services/auth.service';


export class ServiceDetail {
  public versions: {}
  public serviceName: string
  constructor(public service: {}) { }
}

import { Injectable } from '@angular/core';

@Injectable()
export class ServiceDetailService {

  private sl$: BehaviorSubject<ServiceDetail>
  private sl: ServiceDetail = new ServiceDetail({})

  constructor(private http: HttpClient, private auth: AuthService) { } 

  getServiceDetail(serviceName: string) {
    this.sl$ = new BehaviorSubject<ServiceDetail>(new ServiceDetail({}))
    this.sl.serviceName = serviceName
    this.getService(serviceName).subscribe(data => {
      // Read the result field from the JSON response.
      this.sl.service = data['service'];
      this.sl$.next(this.sl)
      this.sl$.complete()
    });
    return this.sl$
  }

  getService(serviceName: string) {
    return this.http.get('/ecs-deploy/api/v1/service/describe/'+serviceName, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }

  getVersions() {
    return this.http.get('/ecs-deploy/api/v1/service/describe/'+this.sl.serviceName+'/versions', {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  getDeployment(version) {
    return this.http.get('/ecs-deploy/api/v1/deploy/get/'+this.sl.serviceName+'/'+version, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  deploy(serviceName, version) {
    return this.http.post('/ecs-deploy/api/v1/deploy/'+serviceName+'/'+version, {}, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
}
