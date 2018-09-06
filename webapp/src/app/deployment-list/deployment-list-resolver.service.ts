import { Component, OnInit } from '@angular/core';
import {HttpClient, HttpHeaders } from '@angular/common/http';

import { AuthService } from '../services/auth.service';
import { environment } from '../../environments/environment';

import { Injectable }             from '@angular/core';
import { Observable }             from 'rxjs';
import { Router, Resolve, RouterStateSnapshot,
         ActivatedRouteSnapshot } from '@angular/router';


import { DeploymentList, DeploymentListService }  from './deployment-list.service';


@Injectable()
export class DeploymentListResolver implements Resolve<DeploymentList> {

  constructor(private ds: DeploymentListService, private router: Router) {}

  resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<DeploymentList> {
    return this.ds.getDeploymentList("")
  }
  
}
