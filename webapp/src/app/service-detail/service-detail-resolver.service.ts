import { Component, OnInit } from '@angular/core';
import {HttpClient, HttpHeaders } from '@angular/common/http';

import { AuthService } from '../services/auth.service';

import { Injectable }             from '@angular/core';
import { Observable }             from 'rxjs';
import { Router, Resolve, RouterStateSnapshot,
         ActivatedRouteSnapshot } from '@angular/router';


import { ServiceDetail, ServiceDetailService }  from './service-detail.service';


@Injectable()
export class ServiceDetailResolver implements Resolve<ServiceDetail> {

  constructor(private ds: ServiceDetailService, private router: Router) {}

  resolve(route: ActivatedRouteSnapshot, state: RouterStateSnapshot): Observable<ServiceDetail> {
    return this.ds.getServiceDetail(route.params.serviceName)
  }
  
}
