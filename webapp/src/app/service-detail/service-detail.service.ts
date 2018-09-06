


import { BehaviorSubject } from 'rxjs';
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
  listParameters() {
    return this.http.get('/ecs-deploy/api/v1/service/parameter/'+this.sl.serviceName+'/list', {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  putParameter(data) {
    return this.http.post('/ecs-deploy/api/v1/service/parameter/'+this.sl.serviceName+'/put', data, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  deleteParameter(serviceName, selectedParameter) {
    return this.http.post('/ecs-deploy/api/v1/service/parameter/'+serviceName+'/delete/' + selectedParameter, {}, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  setDesiredCount(data) {
    return this.http.post('/ecs-deploy/api/v1/service/scale/'+this.sl.serviceName+'/' + data.desiredCount, {}, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  getTaskDefinition() {
    return this.http.get('/ecs-deploy/api/v1/service/describe/'+this.sl.serviceName+'/taskdefinition', {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  runTask(data) {
    return this.http.post('/ecs-deploy/api/v1/service/runtask/'+this.sl.serviceName, data, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  describeTasks() {
    return this.http.get('/ecs-deploy/api/v1/service/describe/'+this.sl.serviceName+'/tasks', {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  getServiceLog(params) {
    return this.http.get('/ecs-deploy/api/v1/service/log/'+this.sl.serviceName+'/get/'+params.taskArn+'/'+params.containerName+'/'+params.start+'/'+params.end, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  putAutoscaling(data) {
    return this.http.post('/ecs-deploy/api/v1/service/autoscaling/'+this.sl.serviceName+'/put', data, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  getAutoscaling() {
    return this.http.get('/ecs-deploy/api/v1/service/autoscaling/'+this.sl.serviceName+'/get', {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  deleteAutoscalingPolicy(serviceName, selectedItem) {
    return this.http.post('/ecs-deploy/api/v1/service/autoscaling/'+serviceName+'/delete/'+selectedItem, {}, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
  disableAutoscaling(serviceName) {
    return this.http.post('/ecs-deploy/api/v1/service/autoscaling/'+serviceName+'/delete', {}, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
  }
}
