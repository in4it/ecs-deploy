


import { BehaviorSubject } from 'rxjs';
import {HttpClient, HttpHeaders } from '@angular/common/http';
import { AuthService } from '../services/auth.service';
import { environment } from '../../environments/environment';


export class DeploymentList {
  constructor(public deployments: string[], public services: string[]) { }
}

import { Injectable } from '@angular/core';

@Injectable()
export class DeploymentListService {

  private dl$: BehaviorSubject<DeploymentList>
  private dl: DeploymentList = new DeploymentList([], [])

  constructor(private http: HttpClient, private auth: AuthService) { } 

  dateOptions = { year: 'numeric', month: 'numeric', day: 'numeric', hour: "2-digit", minute: "2-digit", second: "2-digit", timeZoneName: 'short'};

  getDeploymentList(serviceName: string) {
    this.dl$ = new BehaviorSubject<DeploymentList>(new DeploymentList([], []))
    this.dl.deployments = []
    this.getDeployments(serviceName)
    this.getServices()
    return this.dl$
  }

  getDeployments(serviceName: string) {
    var url
    if(serviceName == "") {
      url = "/ecs-deploy/api/v1/deploy/list"
    } else {
      url = "/ecs-deploy/api/v1/deploy/list/" + serviceName
    }
    this.http.get(url, {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())})
      .subscribe(data => {
      // Read the result field from the JSON response.
      this.dl.deployments = data["deployments"]
      for (let deployment of this.dl.deployments) {
        deployment["Date"] = new Date(deployment["Time"]).toLocaleString("en-US", this.dateOptions)
        var s = deployment["TaskDefinitionArn"].split('/')
        if(s.length > 1){
          deployment["TaskDefinitionVersion"] = s[1]
        }
      }
      this.dl.deployments.sort(function(a,b) {return (a["Time"] > b["Time"]) ? -1 : ((b["Time"] > a["Time"]) ? 1 : 0);} ); 
      if(this.dl.services.length > 0) {
        this.dl$.next(this.dl)
        this.dl$.complete()
        console.log("getDeployment: complete()")
      } else {
        console.log("getDeployments: dl$ complete not triggered")
      }
    })
  }
  getServices() {
    this.http.get('/ecs-deploy/api/v1/service/list', {headers: new HttpHeaders().set('Authorization', "Bearer " + this.auth.getToken())}).subscribe(data => {
      // Read the result field from the JSON response.
      this.dl.services = data['services'];
      if(this.dl.deployments.length > 0) {
        this.dl$.next(this.dl)
        this.dl$.complete()
        console.log("getServices: complete()")
      } else {
        console.log("getServices: dl$ complete not triggered")
      }
    });
    
  }
}
